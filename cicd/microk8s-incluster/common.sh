#!/bin/bash

if [[ "$1" == "init" ]]; then
  pull_dockers
fi

hn="netns"
pid=""
vrn="/var/run/"
hexec="sudo ip netns exec "
dexec="sudo docker exec -i "
hns="sudo ip netns "
hexist="$vrn$hn"
lxdocker="ghcr.io/loxilb-io/loxilb:latest"
var=$(lsb_release -r | cut -f2)
if [[ $var == *"22.04"* ]];then
  lxdocker="ghcr.io/loxilb-io/loxilb:latestU22"
fi

loxilbs=()

## Given a docker name(arg1), return its pid
get_docker_pid() {
  id=`docker ps -f name=$1| grep -w $1 | cut  -d " "  -f 1 | grep -iv  "CONTAINER"`
  pid=`docker inspect -f '{{.State.Pid}}' $id`
}

## Pull all necessary dockers for testbed
pull_dockers() {
  ## loxilb docker
  docker pull $lxdocker
  ## Host docker 
  docker pull eyes852/ubuntu-iperf-test:0.5
  ## BGP host docker
  docker pull ewindisch/quagga
  ## Keepalive docker
  docker pull osixia/keepalived:2.0.20
}

## Creates a docker host
## arg1 - "loxilb"|"host"
## arg2 - instance-name
spawn_docker_host() {
  POSITIONAL_ARGS=()
  local bpath
  local kpath
  local ka
  local bgp
  while [[ $# -gt 0 ]]; do
  case "$1" in
    -t | --dock-type )
      dtype="$2"
      shift 2
      ;;
    -d | --dock-name )
      dname="$2"
      shift 2
      ;;
    -b | --with-bgp )
      if [[ "$2" == "yes" ]]; then
          bgp=$2
      fi
      shift 2
      ;;
    -c | --bgp-config )
      bpath="$2"
      bgp="yes"
      shift 2
      ;;
    -k | --with-ka )
      ka="in"
      if [[ "$2" == "out" ]]; then
          ka=$2
      fi
      shift 2
      ;;
    -d | --ka-config )
      kpath="$2"
      if [[ -z ${ka+x} ]]; then
        ka="in"
      fi
      shift 2
      ;;
    -*|--*)
      echo "Unknown option $1"
      exit
      ;;
  esac
  done  
  set -- "${POSITIONAL_ARGS[@]}" # restore positional parameters
  echo "Spawning $dname($dtype)" >&2 
  if [[ "$dtype" == "loxilb" ]]; then
    loxilbs+=("$dname")
    if [[ "$pick_config" == "yes" ]]; then
        echo "$dname will pick config from $(pwd)/${dname}_config"
        loxilb_config="-v $(pwd)/${dname}_config:/etc/loxilb/"
    fi
    if [[ "$bgp" == "yes" ]]; then
      bgp_opts="-b"
      if [[ ! -z "$bpath" ]]; then
        bgp_conf="-v $bpath:/etc/gobgp/"
      fi
    fi
    if [[ "$dname" == "llb1" ]]; then
      cluster_opts=" --cluster=172.17.0.3 --self=0"
    elif [[ "$dname" == "llb2" ]]; then
      cluster_opts=" --cluster=172.17.0.2 --self=1"
    fi

    if [[ ! -z ${ka+x} ]]; then
      sudo mkdir -p /etc/shared/$dname/
      if [[ "$ka" == "in" ]];then
        ka_opts="-k in"
        if [[ ! -z "$kpath" ]]; then
            ka_conf="-v $kpath:/etc/keepalived/" 
        fi
      fi
      docker run -u root --cap-add SYS_ADMIN   --restart unless-stopped --privileged -dt --entrypoint /bin/bash $bgp_conf -v /dev/log:/dev/log -v /etc/shared/$dname:/etc/shared $loxilb_config $ka_conf --name $dname $lxdocker
      docker exec -dt $dname /root/loxilb-io/loxilb/loxilb $bgp_opts $cluster_opts $ka_opts

      if [[ "$ka" == "out" ]];then
        ka_opts="-k out"
        if [[ ! -z "$kpath" ]]; then
            ka_conf="-v $kpath:/container/service/keepalived/assets/" 
        fi

        docker run -u root --cap-add SYS_ADMIN   --restart unless-stopped --privileged -dit --network=container:$dname $ka_conf -v /etc/shared/$dname:/etc/shared --name ka_$dname osixia/keepalived:2.0.20
      fi
    else
      docker run -u root --cap-add SYS_ADMIN   --restart unless-stopped --privileged -dt --entrypoint /bin/bash $bgp_conf -v /dev/log:/dev/log $loxilb_config --name $dname $lxdocker $bgp_opts
      docker exec -dt $dname /root/loxilb-io/loxilb/loxilb $bgp_opts $cluster_opts
    fi
  elif [[ "$dtype" == "host" ]]; then
    if [[ ! -z "$bpath" ]]; then
      bgp_conf="--volume $bpath:/etc/quagga" 
    fi
    if [[ "$bgp" == "yes" || ! -z "$bpath" ]]; then
      docker run -u root --cap-add SYS_ADMIN  --restart unless-stopped --privileged -dit $bgp_conf --name $dname ewindisch/quagga
    else
      docker run -u root --cap-add SYS_ADMIN -dit --name $dname eyes852/ubuntu-iperf-test:0.5
    fi
  fi

  pid=""

  sleep 2
  get_docker_pid $dname
  echo $pid
  if [ ! -f "$hexist/$dname" -a "$pid" != "" ]; then
    sudo mkdir -p /var/run/netns  
    sudo touch /var/run/netns/$dname
    #echo "sudo mount -o bind /proc/$pid/ns/net /var/run/netns/$2"
    sudo mount -o bind /proc/$pid/ns/net /var/run/netns/$dname
  fi

  $hexec $dname ifconfig lo up
  $hexec $dname sysctl net.ipv6.conf.all.disable_ipv6=1 2>&1 >> /dev/null
  #$hexec $dname sysctl net.ipv4.conf.all.arp_accept=1 2>&1 >> /dev/null
  $hexec $dname sysctl net.ipv4.conf.eth0.arp_ignore=2 2>&1 >> /dev/null
}

## Deletes a docker host
## arg1 - hostname 
delete_docker_host() {
  id=`docker ps -f name=$1| grep -w $1 | cut  -d " "  -f 1 | grep -iv  "CONTAINER"`
  if [ "$id" != "" ]; then
    docker stop $1 2>&1 >> /dev/null
    hd="true"
    ka=`docker ps -f name=ka_$1| grep -w ka_$1 | cut  -d " "  -f 1 | grep -iv  "CONTAINER"`
    loxilbs=( "${loxilbs[@]/$1}" )
    if [ "$ka" != "" ]; then
      docker stop ka_$1 2>&1 >> /dev/null
      docker rm ka_$1 2>&1 >> /dev/null
    fi
  fi
  if [ -f "$hexist/$1" ]; then
    $hns del $1
    sudo rm -fr "$hexist/$1" 2>&1 >> /dev/null
  fi
  if [ "$id" != "" ]; then
    docker rm $1 2>&1 >> /dev/null
  fi
}

## Connects two docker hosts
## arg1 - hostname1 
## arg2 - hostname2 
## arg3 - mtu
connect_docker_hosts() {
  link1=e$1$2
  link2=e$2$1

  mtu="9000"
  if [[ $# -gt 2 ]]; then
    mtu=$3
  fi

  #echo $link1 $link2
  sudo ip -n $1 link add $link1 type veth peer name $link2 netns $2
  sudo ip -n $1 link set $link1 mtu $mtu up
  sudo ip -n $2 link set $link2 mtu $mtu up
}

## arg1 - hostname1 
## arg2 - hostname2 
disconnect_docker_hosts() {
  link1=e$1$2
  link2=e$2$1
  #  echo $link1 $link2
  if [ -f "$hexist/$1" ]; then
    ifexist1=`sudo ip -n $1 link show $link1 | grep -w $link1`
    if [ "chk$ifexist1" != "chk" ]; then
      sudo ip -n $1 link set $link1 down 2>&1 >> /dev/null
      sudo ip -n $1 link del $link1 2>&1 >> /dev/null
    fi
  fi

  if [ -f "$hexist/$2" ]; then
    ifexist2=`sudo ip -n $2 link show | grep -w $link2`
    if [ "chk$ifexist2" != "chk" ]; then
      sudo ip -n $2 link set $link2 down 2>&1 >> /dev/null
      sudo ip -n $2 link del $link2 2>&1 >> /dev/null
    fi
  fi
}

## arg1 - hostname1 
## arg2 - hostname2 
## arg3 - ip_addr
## arg4 - gw
config_docker_host() {
  POSITIONAL_ARGS=()
  while [[ $# -gt 0 ]]; do
    case $1 in
        --host1)
            local h1="$2"
            shift
            shift
            ;;
        --host2)
            local h2="$2"
            shift
            shift
            ;;
        --ptype)
            local ptype="$2"
            shift
            shift
            ;;
        --id)
            local xid="$2"
            shift
            shift
            ;;
        --addr)
            local addr="$2"
            shift
            shift
            ;;
        --gw)
            local gw="$2"
            shift
            shift
            ;;
        -*|--*)
            echo "Unknown option $1"
            exit 1
            ;;
        *)
            POSITIONAL_ARGS+=("$1") # save positional arg
            shift # past argument
            ;;
    esac
  done
  set -- "${POSITIONAL_ARGS[@]}" # restore positional parameters

  link1=e$h1$h2
  link2=e$h2$h1
  #echo "$h1:$link1->$h2:$link2"

  #if [[ -n "${loxilbs[$h1]}" && "$pick_config" == "yes" ]]; then
  if [[ ${loxilbs[*]} =~ (^|[[:space:]])$h1($|[[:space:]]) && "$pick_config" == "yes" ]]; then
    return
  fi

  if [[ "$ptype" == "phy" ]]; then
    sudo ip -n $h1 addr add $addr dev $link1
  elif [[ "$ptype" == "vlan" ]]; then
    sudo ip -n $h1 addr add $addr dev vlan$xid
  elif [[ "$ptype" == "vxlan" ]]; then
    sudo ip -n $h1 addr add $addr dev vxlan$xid
  elif [[ "$ptype" == "trunk" ]]; then
    trunk="bond$xid"
    sudo ip -n $h1 link set $link1 down
    sudo ip -n $h1 link add $trunk type bond
    sudo ip -n $h1 link set $link1 master $trunk
    sudo ip -n $h1 link set $link1 up
    sudo ip -n $h1 link set $trunk up

    sudo ip -n $h2 link set $link2 down
    sudo ip -n $h2 link add $trunk type bond
    sudo ip -n $h2 link set $link2 master $trunk
    sudo ip -n $h2 link set $link2 up
    sudo ip -n $h2 link set $trunk up

    sudo ip -n $h1 addr add $addr dev bond$xid
    if [[ "$gw" != "" ]]; then
      sudo ip -n $h2 addr add $gw/24 dev bond$xid
      sudo ip -n $h1 route add default via $gw proto static
    fi
  else
    echo "Check port-type"
  fi

  if [[ "$gw" != "" ]]; then
    sudo ip -n $h1 route del default 2>&1 >> /dev/null
    sudo ip -n $h1 route add default via $gw
  fi
}

## arg1 - hostname1 
## arg2 - hostname2 
## arg3 - vlan
## arg4 - tagged/untagged
create_docker_host_vlan() {
  local addr=""
  POSITIONAL_ARGS=()
  while [[ $# -gt 0 ]]; do
    case $1 in
        --host1)
            local h1="$2"
            shift
            shift
            ;;
        --host2)
            local h2="$2"
            shift
            shift
            ;;
        --ptype)
            local ptype="$2"
            shift
            shift
            ;;
        --id)
            local vid="$2"
            shift
            shift
            ;;
        --addr)
            addr="$2"
            shift
            shift
            ;;
        -*|--*)
            echo "Unknown option $1"
            exit 1
            ;;
        *)
            POSITIONAL_ARGS+=("$1") # save positional arg
            shift # past argument
            ;;
    esac
  done

  if [[ ${loxilbs[*]} =~ (^|[[:space:]])$h1($|[[:space:]]) && "$pick_config" == "yes" ]]; then
    return
  fi

  set -- "${POSITIONAL_ARGS[@]}" # restore positional parameters
  link1=e$h1$h2
  link2=e$h2$h1

  #echo "$h1:$link1->$h2:$link2"

  if [[ "$ptype" == "tagged" ]]; then
      brport="$link1.$vid"
      sudo ip -n $h1 link add link $link1 name $brport type vlan id $vid
      sudo ip -n $h1 link set $brport up
  else
      brport=$link1
  fi
    
  sudo ip -n $h1 link add vlan$vid type bridge 2>&1 | true
  sudo ip -n $h1 link set $brport master vlan$vid
  sudo ip -n $h1 link set vlan$vid up
  if [[ "$addr" != "" ]]; then
    sudo ip -n $h1 addr add $addr dev vlan$vid
  fi
}

## arg1 - hostname1 
## arg2 - hostname2 
## arg3 - vxlan-id
## arg4 - phy/vlan
## arg5 - local ip if arg4 is phy/vlan-id if arg4 is vlan
## arg6 - local ip if arg4 is vlan
create_docker_host_vxlan() {
  POSITIONAL_ARGS=()
  while [[ $# -gt 0 ]]; do
    case $1 in
        --host1)
            local h1="$2"
            shift
            shift
            ;;
        --host2)
            local h2="$2"
            shift
            shift
            ;;
        --uif)
            local uifType="$2"
            shift
            shift
            ;;
        --vid)
            local vid="$2"
            shift
            shift
            ;;
        --pvid)
            local pvid="$2"
            shift
            shift
            ;;
        --id)
            local vxid="$2"
            shift
            shift
            ;;
        --ep)
            local ep="$2"
            shift
            shift
            ;;
        --lip)
            local lip="$2"
            shift
            shift
            ;;
        -*|--*)
            echo "Unknown option $1"
            exit 1
            ;;
        *)
            POSITIONAL_ARGS+=("$1") # save positional arg
            shift # past argument
            ;;
    esac
  done

  if [[ ${loxilbs[*]} =~ (^|[[:space:]])$h1($|[[:space:]]) && "$pick_config" == "yes" ]]; then
    return
  fi

  set -- "${POSITIONAL_ARGS[@]}" # restore positional parameters
  link1=e$h1$h2
  link2=e$h2$h1

  #echo "$h1:$link1->$h2:$link2"

  if [[ "$uifType" == "phy" ]]; then
    sudo ip -n $h1 link add vxlan$vxid type vxlan id $vxid local $lip dev $link1 dstport 4789
    sudo ip -n $h1 link set vxlan$vxid up
  elif [[ "$uifType" == "vlan" ]]; then
    sudo ip -n $h1 link add vxlan$vxid type vxlan id $vxid local $lip dev vlan$vid dstport 4789
    sudo ip -n $h1 link set vxlan$vxid up
  fi

  if [[ "$pvid" != "" ]]; then
    sudo ip -n $h1 link add vlan$pvid type bridge 2>&1 | true
    sudo ip -n $h1 link set vxlan$vxid master vlan$pvid
    sudo ip -n $h1 link set vlan$pvid up
  fi

  if [[ "$ep" != "" ]]; then
    sudo bridge -n $h1 fdb append 00:00:00:00:00:00 dst $ep dev vxlan$vxid
  fi
  
}

## arg1 - hostname1 
## arg2 - hostname2 
create_docker_host_cnbridge() {
  POSITIONAL_ARGS=()
  while [[ $# -gt 0 ]]; do
    case $1 in
        --host1)
            local h1="$2"
            shift
            shift
            ;;
        --host2)
            local h2="$2"
            shift
            shift
            ;;
        -*|--*)
            echo "Unknown option $1"
            exit 1
            ;;
        *)
            POSITIONAL_ARGS+=("$1") # save positional arg
            shift # past argument
            ;;
    esac
  done

  if [[ ${loxilbs[*]} =~ (^|[[:space:]])$h1($|[[:space:]]) && "$pick_config" == "yes" ]]; then
    return
  fi

  set -- "${POSITIONAL_ARGS[@]}" # restore positional parameters
  link1=e$h1$h2
  link2=e$h2$h1

  #echo "$h1:$link1->$h2:$link2"

  brport=$link1
    
  sudo ip -n $h1 link add br$h1 type bridge 2>&1 | true
  sudo ip -n $h1 link set $brport master br$h1
  sudo ip -n $h1 link set br$h1 up
}

#Arg1: host name
#Arg2: --<proto>:<iport>:<oport>
#Arg3: --endpoints:<ip>:<weight>,..
function create_lb_rule() {
  if [[ ${loxilbs[*]} =~ (^|[[:space:]])$1($|[[:space:]]) && "$pick_config" == "yes" ]]; then
    return
  fi
  args=( "$@" )
  args=( "${args[@]/$1}" )
  echo "$1: loxicmd create lb ${args[*]}"
  $dexec $1 loxicmd create lb ${args[*]}

  hook=$($dexec $1 tc filter show dev eth0 ingress | grep tc_packet_func)
  if [[ $hook != *"tc_packet_func"* ]]; then
    echo "ERROR : No hook point found";
    exit 1
  fi
}

#Arg1: host name
#Arg2: <prefix/mask>
#Arg3: <nexthop-ip>
function add_route() {
  if [[ ${loxilbs[*]} =~ (^|[[:space:]])$1($|[[:space:]]) && "$pick_config" == "yes" ]]; then
    return
  fi
  echo "$1: ip route add $2 via $3 proto static"
  $hexec $1 ip route add $2 via $3 proto static
}
