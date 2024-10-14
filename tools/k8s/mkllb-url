#!/bin/bash

usage() {
    echo "Usage: $0 -a <ip-addr> -z <zone> -t <type>"
    echo "       $0 -a <ip-addr> -z <zone> -t <type> -d"
}

if [[ $# -gt 7 ]] || [[ $# -lt 6 ]]; then
   usage
   exit
fi

if [[ ! -f /usr/local/sbin/kubectl ]]; then
    apt-get update && apt-get install -y curl
    curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
    chmod +x kubectl
    sudo mv kubectl /usr/local/sbin/kubectl
fi

addr=""
zone="llb"
utype="default"
cmd="apply"

while getopts a:z:t:x opt 
do
    case "${opt}" in
        a) addr=${OPTARG};;
        z) zone=${OPTARG};;
        t) utype=${OPTARG};;
        x) cmd="delete";;
        ?) usage;exit;;
    esac
done

echo "============"
echo "Applying CRD"
echo "============"
echo addr $addr
echo zone $zone
echo utype $utype
echo cmd $cmd
echo "============"

cat <<EOF | kubectl ${cmd} -f -
apiVersion: "loxiurl.loxilb.io/v1"
kind: LoxiURL
metadata:
  name: llb-${addr}
spec:
  loxiURL: http://${addr}:11111
  zone: llb
  type: ${utype}
EOF
