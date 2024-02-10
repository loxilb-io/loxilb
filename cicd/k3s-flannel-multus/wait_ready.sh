#!/bin/bash

function wait_cluster_ready {
    Res=$(sudo kubectl get pods -A |
    while IFS= read -r line; do
        if [[ "$line" != *"Running"* && "$line" != *"READY"* ]]; then
            echo "not ready"
            return
        fi
    done)
    if [[ $Res == *"not ready"* ]]; then
        return 1
    fi
    return 0
}

function wait_cluster_ready_full {
  i=1
  nr=0
  for ((;;)) do
    wait_cluster_ready
    nr=$?
    if [[ $nr == 0 ]]; then
        echo "Cluster is ready"
        break
    fi
    i=$(( $i + 1 ))
    if [[ $i -ge 40 ]]; then
        echo "Cluster is not ready.Giving up"
        exit 1
    fi
    echo "Cluster is not ready...."
    sleep 10
  done
}

wait_cluster_ready_full
