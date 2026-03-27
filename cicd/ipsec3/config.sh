#!/bin/bash
source ../common.sh

runtime_lxdocker="local/loxilb-ci-ipsec:prep"
prep_container="loxilb-ipsec-prep"

cleanup_prep_container() {
	docker rm -f "$prep_container" >/dev/null 2>&1 || true
}

prepare_runtime_loxilb_image() {
	if docker image inspect "$runtime_lxdocker" >/dev/null 2>&1; then
		lxdocker="$runtime_lxdocker"
		return
	fi

	echo "Preparing one-time runtime image: $runtime_lxdocker"
	cleanup_prep_container
	docker run -dt --entrypoint /bin/bash --name "$prep_container" "$lxdocker"
	docker exec -i "$prep_container" bash -lc "apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y iptables strongswan strongswan-swanctl systemctl iputils-ping curl"
	docker commit "$prep_container" "$runtime_lxdocker" >/dev/null
	cleanup_prep_container
	lxdocker="$runtime_lxdocker"
}

trap cleanup_prep_container EXIT
prepare_runtime_loxilb_image

echo "#########################################"
echo "Spawning all hosts"
echo "#########################################"

spawn_docker_host --dock-type loxilb --dock-name llb1
spawn_docker_host --dock-type loxilb --dock-name llb2
spawn_docker_host --dock-type host --dock-name lh1
spawn_docker_host --dock-type host --dock-name rh1

echo "#########################################"
echo "Connecting and configuring  hosts"
echo "#########################################"

#Left config
connect_docker_hosts lh1 llb1
connect_docker_hosts llb1 llb2

config_docker_host --host1 lh1 --host2 llb1 --ptype phy --addr 192.168.10.175/24 --gw 192.168.10.1
config_docker_host --host1 llb1 --host2 lh1 --ptype phy --addr 192.168.10.1/24
config_docker_host --host1 llb1 --host2 llb2 --ptype phy --addr 7.7.7.1/24
config_docker_host --host1 llb2 --host2 llb1 --ptype phy --addr 7.7.7.2/24

#Right Config
connect_docker_hosts rh1 llb2

config_docker_host --host1 rh1 --host2 llb2 --ptype phy --addr 192.168.10.10/24 --gw 192.168.10.254
config_docker_host --host1 llb2 --host2 rh1 --ptype phy --addr 192.168.10.254/24

#xfrm Config(Left)
$dexec llb1 ip link add vti100 type vti key 100 remote 7.7.7.2 local 7.7.7.1
$dexec llb1 ip link set vti100 up
$dexec llb1 ip addr add 77.77.77.1/24 remote 77.77.77.2/24 dev vti100
$dexec llb1 sysctl -w "net.ipv4.conf.vti100.disable_policy=1"
$dexec llb1 sysctl -w "net.ipv4.conf.ellb1lh1.proxy_arp=1"

$dexec llb1 ip route add 192.168.10.200/32 via 77.77.77.2

#xfrm Config(Right)
$dexec llb2 ip link add vti100 type vti key 100 remote 7.7.7.1 local 7.7.7.2
$dexec llb2 ip link set vti100 up
$dexec llb2 ip addr add 77.77.77.2/24 remote 77.77.77.1/24 dev vti100
$dexec llb2 sysctl -w "net.ipv4.conf.vti100.disable_policy=1"
$dexec llb2 sysctl -w "net.ipv4.conf.ellb2rh1.proxy_arp=1"

$dexec llb2 ip addr add 192.168.10.200/32 dev lo
$dexec llb2 ip route add 192.168.10.175/32 via 77.77.77.1 dev vti100
$dexec llb2 loxicmd create lb 192.168.10.200 --tcp=2020:8080 --endpoints=192.168.10.10:1 --mode=onearm

docker cp llb1_ipsec_config/ipsec.conf llb1:/etc/
docker cp llb1_ipsec_config/ipsec.secrets llb1:/etc/
docker cp llb1_ipsec_config/charon.conf llb1:/etc/strongswan.d/
$dexec llb1 systemctl restart strongswan-starter

docker cp llb2_ipsec_config/ipsec.conf llb2:/etc/
docker cp llb2_ipsec_config/ipsec.secrets llb2:/etc/
docker cp llb2_ipsec_config/charon.conf llb2:/etc/strongswan.d/
$dexec llb2 systemctl restart strongswan-starter
