#!/bin/bash

# Wait until the cluster is usable: every node Ready, and every pod either
# Running with all of its containers ready, or finished successfully.

LastErr=""

function wait_cluster_ready {
    local output

    # A failing kubectl must not read as success. This used to pipe kubectl
    # straight into the loop below, so an API error left the loop with nothing
    # to read and the cluster was reported ready. That is how a k3s server that
    # never started still printed "Cluster is ready".
    if ! output=$(sudo kubectl $KUBECONFIG get nodes --no-headers 2>&1); then
        LastErr="kubectl get nodes failed:"$'\n'"$output"
        return 1
    fi
    if [[ -z "$output" ]]; then
        LastErr="kubectl get nodes returned no nodes"
        return 1
    fi

    local notready
    notready=$(echo "$output" | awk '$2 !~ /^Ready/ { print }')
    if [[ -n "$notready" ]]; then
        LastErr="node(s) not Ready:"$'\n'"$notready"
        return 1
    fi

    if ! output=$(sudo kubectl $KUBECONFIG get pods -A --no-headers 2>&1); then
        LastErr="kubectl get pods failed:"$'\n'"$output"
        return 1
    fi

    # Columns are NAMESPACE NAME READY STATUS RESTARTS AGE. Matching on the
    # string "Running" alone accepted a pod whose containers were not ready
    # yet, and rejected Jobs that had legitimately finished.
    notready=$(echo "$output" | awk '
        $4 == "Completed" || $4 == "Succeeded" { next }
        $4 != "Running" { print; next }
        { split($3, r, "/"); if (r[1] != r[2]) print }
    ')
    if [[ -n "$notready" ]]; then
        LastErr="pod(s) not ready:"$'\n'"$notready"
        return 1
    fi

    return 0
}

function wait_cluster_ready_full {
  i=1
  for ((;;)) do
    if wait_cluster_ready; then
        echo "Cluster is ready"
        break
    fi
    i=$(( $i + 1 ))
    if [[ $i -ge 40 ]]; then
        echo "Cluster is not ready.Giving up"
        # Say what was still wrong. Giving up silently left nothing in the
        # run log to work from.
        echo "--- what was still failing ---"
        echo "$LastErr"
        echo "--- nodes ---"
        sudo kubectl $KUBECONFIG get nodes -o wide 2>&1
        echo "--- pods ---"
        sudo kubectl $KUBECONFIG get pods -A -o wide 2>&1
        exit 1
    fi
    echo "Cluster is not ready...."
    sleep 10
  done
}
