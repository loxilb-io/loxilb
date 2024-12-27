#!/bin/bash

source ../common.sh

echo "#########################################"
echo "Spawning all hosts"
echo "#########################################"

spawn_docker_host --dock-type loxilb --dock-name llb1
spawn_docker_host --dock-type host --dock-name l3h1
spawn_docker_host --dock-type host --dock-name l3h2
spawn_docker_host --dock-type host --dock-name l3h3
spawn_docker_host --dock-type host --dock-name l2h1
spawn_docker_host --dock-type host --dock-name l2h2
spawn_docker_host --dock-type host --dock-name l2h3
spawn_docker_host --dock-type host --dock-name l2h4
spawn_docker_host --dock-type host --dock-name l2h5
spawn_docker_host --dock-type host --dock-name l2h6
spawn_docker_host --dock-type host --dock-name l2vxh1
spawn_docker_host --dock-type host --dock-name l2vxh2
spawn_docker_host --dock-type host --dock-name l2vxh3
spawn_docker_host --dock-type host --dock-name l2vxh4
spawn_docker_host --dock-type host --dock-name l2vxh5
spawn_docker_host --dock-type host --dock-name l2vxh6
spawn_docker_host --dock-type host --dock-name l2vxh7
spawn_docker_host --dock-type host --dock-name l2vxh8
spawn_docker_host --dock-type host --dock-name l2vxh9
spawn_docker_host --dock-type host --dock-name l3vxh1
spawn_docker_host --dock-type host --dock-name l3vxh2
spawn_docker_host --dock-type host --dock-name l3vxh3

echo "#########################################"
echo "Connecting and configuring  hosts"
echo "#########################################"


connect_docker_hosts l3h1 llb1
connect_docker_hosts l3h2 llb1
connect_docker_hosts l3h3 llb1
connect_docker_hosts l2h1 llb1
connect_docker_hosts l2h2 llb1
connect_docker_hosts l2h3 llb1
connect_docker_hosts l2h4 llb1
connect_docker_hosts l2h5 llb1
connect_docker_hosts l2h6 llb1
connect_docker_hosts l2vxh1 llb1
connect_docker_hosts l2vxh2 llb1
connect_docker_hosts l2vxh3 llb1
connect_docker_hosts l2vxh4 llb1
connect_docker_hosts l2vxh5 llb1
connect_docker_hosts l2vxh6 llb1
connect_docker_hosts l2vxh7 llb1
connect_docker_hosts l2vxh8 llb1
connect_docker_hosts l2vxh9 llb1
connect_docker_hosts l3vxh1 llb1
connect_docker_hosts l3vxh2 llb1
connect_docker_hosts l3vxh3 llb1


#L3 config
config_docker_host --host1 l3h1 --host2 llb1 --ptype phy --addr 31.31.31.1/24 --gw 31.31.31.254
config_docker_host --host1 l3h2 --host2 llb1 --ptype phy --addr 32.32.32.1/24 --gw 32.32.32.254
config_docker_host --host1 l3h3 --host2 llb1 --ptype phy --addr 33.33.33.1/24 --gw 33.33.33.254
config_docker_host --host1 llb1 --host2 l3h1 --ptype phy --addr 31.31.31.254/24
config_docker_host --host1 llb1 --host2 l3h2 --ptype phy --addr 32.32.32.254/24
config_docker_host --host1 llb1 --host2 l3h3 --ptype phy --addr 33.33.33.254/24

#L2 config
  ## Case 1
  #  Tagged vlan ports
  # l2h1 Config
create_docker_host_vlan --host1 l2h1 --host2 llb1 --id 100 --ptype tagged
config_docker_host --host1 l2h1 --host2 llb1 --ptype vlan --id 100 --addr 100.100.100.1/24 --gw 100.100.100.254
  # l2h2 Config
create_docker_host_vlan --host1 l2h2 --host2 llb1 --id 100 --ptype tagged

config_docker_host --host1 l2h2 --host2 llb1 --ptype vlan --id 100 --addr 100.100.100.2/24 --gw 100.100.100.254
config_docker_host --host1 l2h2 --host2 llb1 --ptype vlan --id 100 --addr 100.100.100.3/24
config_docker_host --host1 l2h2 --host2 llb1 --ptype vlan --id 100 --addr 100.100.100.4/24
config_docker_host --host1 l2h2 --host2 llb1 --ptype vlan --id 100 --addr 100.100.100.5/24
config_docker_host --host1 l2h2 --host2 llb1 --ptype vlan --id 100 --addr 100.100.100.6/24
config_docker_host --host1 l2h2 --host2 llb1 --ptype vlan --id 100 --addr 100.100.100.7/24
config_docker_host --host1 l2h2 --host2 llb1 --ptype vlan --id 100 --addr 100.100.100.8/24
config_docker_host --host1 l2h2 --host2 llb1 --ptype vlan --id 100 --addr 100.100.100.9/24
config_docker_host --host1 l2h2 --host2 llb1 --ptype vlan --id 100 --addr 100.100.100.10/24
config_docker_host --host1 l2h2 --host2 llb1 --ptype vlan --id 100 --addr 100.100.100.11/24
config_docker_host --host1 l2h2 --host2 llb1 --ptype vlan --id 100 --addr 100.100.100.12/24
config_docker_host --host1 l2h2 --host2 llb1 --ptype vlan --id 100 --addr 100.100.100.13/24

    #loxilb config
create_docker_host_vlan --host1 llb1 --host2 l2h1 --id 100 --ptype tagged
create_docker_host_vlan --host1 llb1 --host2 l2h2 --id 100 --ptype tagged
config_docker_host --host1 llb1 --host2 l2h1 --ptype vlan --id 100 --addr 100.100.100.254/24

  ## Case 2
  #  Untagged vlan ports
  #  l2h3 Config
create_docker_host_vlan --host1 l2h3 --host2 llb1 --id 101 --ptype untagged
config_docker_host --host1 l2h3 --host2 llb1 --ptype vlan --id 101 --addr 101.101.101.1/24 --gw 101.101.101.254

  #  l2h4 Config
create_docker_host_vlan --host1 l2h4 --host2 llb1 --id 101 --ptype untagged
config_docker_host --host1 l2h4 --host2 llb1 --ptype vlan --id 101 --addr 101.101.101.2/24 --gw 101.101.101.254

  #  loxilb config
create_docker_host_vlan --host1 llb1 --host2 l2h3 --id 101 --ptype untagged
create_docker_host_vlan --host1 llb1 --host2 l2h4 --id 101 --ptype untagged
config_docker_host --host1 llb1 --host2 l2h3 --ptype vlan --id 101 --addr 101.101.101.254/24

  ## Case 3
  #  Tagged and Untagged vlan ports
  #  l2h5 Config
create_docker_host_vlan --host1 l2h5 --host2 llb1 --id 102 --ptype untagged
config_docker_host --host1 l2h5 --host2 llb1 --ptype vlan --id 102 --addr 102.102.102.1/24 --gw 102.102.102.254

  #  l2h6 Config
create_docker_host_vlan --host1 l2h6 --host2 llb1 --id 102 --ptype tagged
config_docker_host --host1 l2h6 --host2 llb1 --ptype vlan --id 102 --addr 102.102.102.2/24 --gw 102.102.102.254

  #  loxilb config
create_docker_host_vlan --host1 llb1 --host2 l2h5 --id 102 --ptype untagged
create_docker_host_vlan --host1 llb1 --host2 l2h6 --id 102 --ptype tagged
config_docker_host --host1 llb1 --host2 l2h5 --ptype vlan --id 102 --addr 102.102.102.254/24


#L2 VxLAN config
  ## Case 1
  # Two Access ports: tagged and Untagged. Underlying interface for VxLAN is phy(untagged)
    #L2vxlan Host1
create_docker_host_vlan --host1 l2vxh1 --host2 llb1 --id 50 --ptype tagged
config_docker_host --host1 l2vxh1 --host2 llb1 --ptype vlan --id 50 --addr 50.50.50.1/24

    #L2vxlan Host2
config_docker_host --host1 l2vxh2 --host2 llb1 --ptype phy --addr 2.2.2.2/24
create_docker_host_vxlan --host1 l2vxh2 --host2 llb1 --id 50 --uif phy --lip 2.2.2.2
config_docker_host --host1 l2vxh2 --host2 llb1 --ptype vxlan --id 50 --addr 50.50.50.2/24
create_docker_host_vxlan --host1 l2vxh2 --host2 llb1 --id 50 --ep 2.2.2.1

    #L2vxlan Host3
create_docker_host_vlan --host1 l2vxh3 --host2 llb1 --id 50 --ptype untagged
config_docker_host --host1 l2vxh3 --host2 llb1 --ptype vlan --id 50 --addr 50.50.50.3/24

    #Loxilb config
create_docker_host_vlan --host1 llb1 --host2 l2vxh1 --id 50 --ptype tagged
create_docker_host_vlan --host1 llb1 --host2 l2vxh3 --id 50 --ptype untagged

config_docker_host --host1 llb1 --host2 l2vxh2 --ptype phy --addr 2.2.2.1/24
create_docker_host_vxlan --host1 llb1 --host2 l2vxh2 --id 50 --uif phy --lip 2.2.2.1 --pvid 50
create_docker_host_vxlan --host1 llb1 --host2 l2vxh2 --id 50 --ep 2.2.2.2


  ## Case 2
  # Two Access ports: tagged and untagged. Underlying interface for VxLAN is VLAN(untagged)
    #L2vxlan Host4
create_docker_host_vlan --host1 l2vxh4 --host2 llb1 --id 51 --ptype tagged
config_docker_host --host1 l2vxh4 --host2 llb1 --ptype vlan --id 51 --addr 51.51.51.1/24

    #L2vxlan Host5
create_docker_host_vlan --host1 l2vxh5 --host2 llb1 --id 3 --ptype untagged
config_docker_host --host1 l2vxh5 --host2 llb1 --ptype vlan --id 3 --addr 3.3.3.2/24
create_docker_host_vxlan --host1 l2vxh5 --host2 llb1 --id 51 --uif vlan --vid 3 --lip 3.3.3.2
config_docker_host --host1 l2vxh5 --host2 llb1 --ptype vxlan --id 51 --addr 51.51.51.2/24
create_docker_host_vxlan --host1 l2vxh5 --host2 llb1 --id 51 --ep 3.3.3.1

    #L2vxlan Host6
create_docker_host_vlan --host1 l2vxh6 --host2 llb1 --id 51 --ptype untagged
config_docker_host --host1 l2vxh6 --host2 llb1 --ptype vlan --id 51 --addr 51.51.51.3/24

    #Loxilb config
create_docker_host_vlan --host1 llb1 --host2 l2vxh4 --id 51 --ptype tagged
create_docker_host_vlan --host1 llb1 --host2 l2vxh6 --id 51 --ptype untagged

create_docker_host_vlan --host1 llb1 --host2 l2vxh5 --id 3 --ptype untagged
config_docker_host --host1 llb1 --host2 l2vxh5 --ptype vlan --id 3 --addr 3.3.3.1/24
create_docker_host_vxlan --host1 llb1 --host2 l2vxh5 --id 51 --uif vlan --vid 3 --lip 3.3.3.1 --pvid 51
create_docker_host_vxlan --host1 llb1 --host2 l2vxh5 --id 51 --ep 3.3.3.2


  ## Case 3
  # Two Access ports: tagged and untagged. Underlying interface for VxLAN is VLAN(tagged)
    #L2vxlan Host7
create_docker_host_vlan --host1 l2vxh7 --host2 llb1 --id 52 --ptype tagged
config_docker_host --host1 l2vxh7 --host2 llb1 --ptype vlan --id 52 --addr 52.52.52.1/24

    #L2vxlan Host8
create_docker_host_vlan --host1 l2vxh8 --host2 llb1 --id 4 --ptype tagged
config_docker_host --host1 l2vxh8 --host2 llb1 --ptype vlan --id 4 --addr 4.4.4.2/24
create_docker_host_vxlan --host1 l2vxh8 --host2 llb1 --id 52 --uif vlan --vid 4 --lip 4.4.4.2
config_docker_host --host1 l2vxh8 --host2 llb1 --ptype vxlan --id 52 --addr 52.52.52.254
create_docker_host_vxlan --host1 l2vxh8 --host2 llb1 --id 52 --ep 4.4.4.1

    #L2vxlan Host9
create_docker_host_vlan --host1 l2vxh9 --host2 llb1 --id 52 --ptype untagged
config_docker_host --host1 l2vxh9 --host2 llb1 --ptype vlan --id 52 --addr 52.52.52.3/24

    #Loxilb config
create_docker_host_vlan --host1 llb1 --host2 l2vxh7 --id 52 --ptype tagged
create_docker_host_vlan --host1 llb1 --host2 l2vxh9 --id 52 --ptype untagged

create_docker_host_vlan --host1 llb1 --host2 l2vxh8 --id 4 --ptype tagged
config_docker_host --host1 llb1 --host2 l2vxh8 --ptype vlan --id 4 --addr 4.4.4.1/24
create_docker_host_vxlan --host1 llb1 --host2 l2vxh8 --id 52 --uif vlan --vid 4 --lip 4.4.4.1 --pvid 52
create_docker_host_vxlan --host1 llb1 --host2 l2vxh8 --id 52 --ep 4.4.4.2


#L3 VxLAN config with parent bridge
  ## Case 1
  # Two Access ports: tagged and Untagged. Underlying interface for VxLAN is phy(untagged)
    #L3vxlan Host1
create_docker_host_vlan --host1 l3vxh1 --host2 llb1 --id 60 --ptype tagged
config_docker_host --host1 l3vxh1 --host2 llb1 --ptype vlan --id 60 --addr 60.60.60.1/24 --gw 60.60.60.254

    #L3vxlan Host2
config_docker_host --host1 l3vxh2 --host2 llb1 --ptype phy --addr 5.5.5.2/24
create_docker_host_vxlan --host1 l3vxh2 --host2 llb1 --id 60 --uif phy --lip 5.5.5.2
config_docker_host --host1 l3vxh2 --host2 llb1 --ptype vxlan --id 60 --addr 60.60.60.2/24 --gw 60.60.60.254
create_docker_host_vxlan --host1 l3vxh2 --host2 llb1 --id 60 --ep 5.5.5.1

    #L3vxlan Host3
create_docker_host_vlan --host1 l3vxh3 --host2 llb1 --id 60 --ptype untagged
config_docker_host --host1 l3vxh3 --host2 llb1 --ptype vlan --id 60 --addr 60.60.60.3/24 --gw 60.60.60.254

    #Loxilb config
create_docker_host_vlan --host1 llb1 --host2 l3vxh1 --id 60 --ptype tagged
create_docker_host_vlan --host1 llb1 --host2 l3vxh3 --id 60 --ptype untagged

config_docker_host --host1 llb1 --host2 l3vxh2 --ptype phy --addr 5.5.5.1/24
create_docker_host_vxlan --host1 llb1 --host2 l3vxh2 --id 60 --uif phy --lip 5.5.5.1 --pvid 60
create_docker_host_vxlan --host1 llb1 --host2 l3vxh2 --id 60 --ep 5.5.5.2
config_docker_host --host1 llb1 --host2 l3vxh2 --ptype vlan --id 60 --addr 60.60.60.254/24

sleep 60
