#!/bin/bash

sudo curl -L git.io/weave -o /usr/local/bin/weave
sudo chmod a+x /usr/local/bin/weave
weave reset --force
sudo rm -fr /usr/local/bin/weave

ip a | grep 'vethwepl.*\@' -oP | while read -r line ; do
    veth=${line::-1}
    if [[ $veth =~ [0-9] ]]; then
      echo check $veth
      pid=$(echo $veth | tr -dc '0-9')
      if ! ps -p $pid > /dev/null; then
        echo deleting $veth
        ip link delete $veth >&2
      else
        echo $veth still running
      fi
    else
      echo $veth veth has no number in it and will not be deleted
    fi
done
