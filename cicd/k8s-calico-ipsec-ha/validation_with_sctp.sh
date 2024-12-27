#!/bin/bash
source ../common.sh
echo k8s-calico-ipsec-ha

if [ "$1" ]; then
  KUBECONFIG="$1"
fi

# Set space as the delimiter
IFS=' '

for((i=0; i<120; i++))
do
  extLB=$(vagrant ssh master -c 'kubectl get svc' 2> /dev/null | grep "tcp-lb-default")
  read -a strarr <<< "$extLB"
  len=${#strarr[*]}
  if [[ $((len)) -lt 6 ]]; then
    echo "Can't find tcp-lb service"
    sleep 1
    continue
  fi 
  if [[ ${strarr[3]} != *"none"* ]]; then
    extIP="$(cut -d'-' -f2 <<<${strarr[3]})"
    break
  fi
  echo "No external LB allocated"
  sleep 1
done

## Any routing updates  ??
#sleep 30

echo Service IP : $extIP
echo $extIP > extIP
echo -e "\nEnd Points List"
echo "******************************************************************************"
vagrant ssh master -c 'kubectl get endpoints -A' 2> /dev/null
echo "******************************************************************************"
echo -e "\nSVC List"
echo "******************************************************************************"
vagrant ssh master -c 'kubectl get svc' 2> /dev/null
echo "******************************************************************************"
echo -e "\nPod List"
echo "******************************************************************************"
vagrant ssh master -c 'kubectl get pods -A' 2> /dev/null
echo "******************************************************************************"
echo -e "\nLB List"
echo -e "\n---- LLB1 ----"
echo "******************************************************************************"
vagrant ssh llb1 -c 'sudo ./loxicmd get lb -o wide' 2> /dev/null
echo "******************************************************************************"
echo -e "\n---- LLB2 ----"
vagrant ssh llb2 -c 'sudo ./loxicmd get lb -o wide' 2> /dev/null
echo "******************************************************************************"
echo -e "\nEP List"
echo -e "\n---- LLB1 ----"
echo "******************************************************************************"
vagrant ssh llb1 -c 'sudo ./loxicmd get ep -o wide' 2> /dev/null
echo "******************************************************************************"
echo -e "\n---- LLB2 ----"
vagrant ssh llb2 -c 'sudo ./loxicmd get ep -o wide' 2> /dev/null
echo "******************************************************************************"
echo -e "\nTEST RESULTS"
echo "******************************************************************************"

master="llb1"
backup="llb2"

state=$(curl -sX 'GET' 'http://192.168.80.252:11111/netlox/v1/config/cistate/all' -H 'accept: application/json')

if [[ $state == *"BACKUP"* ]]; then
  master="llb2"
  backup="llb1"
fi

echo -e "\n MASTER\t: $master"
echo -e " BACKUP\t: $backup\n"

vagrant ssh host -c 'sudo /vagrant/host_validation_with_sctp.sh' 2> /dev/null

sleep 15
echo -e "phase-2 begins..\n"

count=1
sync=0
while [[ $count -le 20 ]] ; do
echo -e "\nStatus at MASTER:$master\n"
vagrant ssh $master -c "sudo ./loxicmd get ct | grep est" 2> /dev/null

echo -e "\nStatus at BACKUP:$backup\n"
vagrant ssh $backup -c "sudo ./loxicmd get ct | grep est" 2> /dev/null

nres1=$(curl -sX 'GET' 'http://192.168.80.252:11111/netlox/v1/config/conntrack/all' -H 'accept: application/json' | grep -ow "\"conntrackState\":\"est\"" | wc -l)
nres2=$(curl -sX 'GET' 'http://192.168.80.253:11111/netlox/v1/config/conntrack/all' -H 'accept: application/json' | grep -ow "\"conntrackState\":\"est\"" | wc -l)

if [[ ! -z $nres1 && $nres1 != 0 && $nres1 == $nres2 ]]; then
    echo -e "\nConnections sync successful!!!\n"
    sync=1
    break;
fi
echo -e "\nConnections sync pending.. Let's wait a little more..\n"
count=$(( $count + 1 ))
sleep 2
done

if [[ $sync == 0 ]]; then
    echo -e "\nConnection Sync failed\n"
    vagrant ssh host -c 'sudo pkill iperf; sudo pkill sctp_test; sudo rm -rf *.out'
    exit 1
fi

echo "Restarting MASTER:$master.."
vagrant ssh $master -c 'sudo docker restart loxilb' 2> /dev/null

sleep 30

sudo rm extIP
vagrant ssh host -c 'sudo /vagrant/host_validation2_with_sctp.sh' 2> /dev/null
code=`cat status.txt`
rm status.txt
exit $code
