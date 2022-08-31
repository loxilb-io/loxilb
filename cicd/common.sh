#!/bin/bash

if [[ "$1" == "init" ]]; then
  pull_dockers
fi

hexec="sudo ip netns exec "
dexec="sudo docker exec -i "
pid=""

## Given a docker name(arg1), return its pid
get_docker_pid() {
  id=`docker ps -f name=$1| grep -w $1 | cut  -d " "  -f 1 | grep -iv  "CONTAINER"`
  pid=`docker inspect -f '{{.State.Pid}}' $id`
}

## Pull all necessary dockers for testbed
pull_dockers() {
  ## loxilb docker
  docker pull ghcr.io/loxilb-io/loxilb:latest
  ## Host docker 
  docker pull eyes852/ubuntu-iperf-test:0.5
}

## arg1 - "loxilb"|"host"
## arg2 - instance-name
spawn_docker_host() {
  pid=""
  if [[ "$1" == "loxilb" ]]; then
    docker run -u root --cap-add SYS_ADMIN   --restart unless-stopped --privileged -dit -v /dev/log:/dev/log --name $2 ghcr.io/loxilb-io/loxilb:latest

  else
    docker run -u root --cap-add SYS_ADMIN -dit --name $2 eyes852/ubuntu-iperf-test:0.5
  fi

  sleep 2
  get_docker_pid $2
  echo $pid
  if [ ! -f "/var/run/netns/$2" -a "$pid" != "" ]; then
    sudo touch /var/run/netns/$2
    echo "sudo mount -o bind /proc/$pid/ns/net /var/run/netns/$2"
    sudo mount -o bind /proc/$pid/ns/net /var/run/netns/$2
  fi

  $hexec $2 ifconfig lo up
  $hexec $2 ifconfig eth0 0
  $hexec $2 sysctl net.ipv6.conf.all.disable_ipv6=1
}

## arg1 - hostname 
delete_docker_host() {
  docker stop $1 2>&1 >> /dev/null
  sudo ip netns del $1 2>&1 >> /dev/null
  sudo rm -fr /var/run/$1 2>&1 >> /dev/null
  docker rm $1 2>&1 >> /dev/null
}

## arg1 - hostname1 
## arg2 - hostname2 
connect_docker_hosts() {
  link1=e$1$2
  link2=e$2$1
  #echo $link1 $link2
  sudo ip -n $1 link add $link1 type veth peer name $link2 netns $2
  sudo ip -n $1 link set $link1 mtu 9000 up
  sudo ip -n $2 link set $link2 mtu 9000 up
}

## arg1 - hostname1 
## arg2 - hostname2 
disconnect_docker_hosts() {
  link1=e$1$2
  link2=e$2$1
  #  echo $link1 $link2
  ifexist=`sudo ip -n $1 link show $link1 | grep -w $link1`
  if [ "$ifexist" != "" ]; then 
    sudo ip -n $1 link set $link1 down 2>&1 >> /dev/null
    sudo ip -n $1 link del $link1 2>&1 >> /dev/null
  fi

  ifexist=`sudo ip -n $2 link show $link2 | grep -w $link2`
  if [ "$ifexist" != "" ]; then 
    sudo ip -n $2 link set $link2 down 2>&1 >> /dev/null
    sudo ip -n $2 link del $link2 2>&1 >> /dev/null
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
            local vid="$2"
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

  if [[ "$ptype" == "phy" ]]; then
    sudo ip -n $h1 addr add $addr dev $link1
  elif [[ "$ptype" == "vlan" ]]; then
    sudo ip -n $h1 addr add $addr dev vlan$vid
  elif [[ "$ptype" == "vxlan" ]]; then
    sudo ip -n $h1 addr add $addr dev vxlan$vid
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

  set -- "${POSITIONAL_ARGS[@]}" # restore positional parameters
  link1=e$h1$h2
  link2=e$h2$h1

  #echo "$h1:$link1->$h2:$link2"

  brport=$link1
    
  sudo ip -n $h1 link add br$h1 type bridge 2>&1 | true
  sudo ip -n $h1 link set $brport master br$h1
  sudo ip -n $h1 link set br$h1 up
}


