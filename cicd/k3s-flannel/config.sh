#!/bin/bash

if [ "$#" -eq 2 ]; then
  ubuntu=$2
  echo "version: $ubuntu"
fi

if [[ "$ubuntu" == "20.04" ]]; then
  export VAGRANT_BOX="sysnet4admin/Ubuntu-k8s"
  export VAGRANT_BOX_VERSION="0.7.1"
elif [[ "$ubuntu" == "22.04" ]]; then
  export VAGRANT_BOX="bento/ubuntu-22.04"
  export VAGRANT_BOX_VERSION=">= 0"
else
  export VAGRANT_BOX="sysnet4admin/Ubuntu-k8s"
  export VAGRANT_BOX_VERSION="0.7.1"
fi
vagrant up k3s
