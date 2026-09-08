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

# ---------------------------------------------------------------------------
# Host-side helpers for Vagrant scenarios. They run kubectl on the master VM
# through "vagrant ssh", so they are meant for config.sh/validation.sh on the
# host, not for scripts inside a VM. Override K3S_KUBECTL_NODE if the master is
# not called master1.

K3S_KUBECTL_NODE=${K3S_KUBECTL_NODE:-master1}

function vm_kubectl {
    local out rc
    # Capture first, then strip the CRs vagrant ssh adds. Piping straight into
    # tr would replace kubectl's exit status with tr's, and callers rely on it.
    out=$(vagrant ssh "$K3S_KUBECTL_NODE" -c "sudo kubectl $*" 2> /dev/null)
    rc=$?
    printf '%s\n' "$out" | tr -d '\r'
    return $rc
}

# wait_for_daemonset <name> [namespace]
# Returns once every pod of the DaemonSet is ready. kubectl create returns as
# soon as the object exists, long before its image is pulled, so a validation
# that starts right after it races the rollout.
function wait_for_daemonset {
    local name=$1
    local ns=${2:-default}
    local attempt=0

    while true; do
        if vm_kubectl -n "$ns" rollout status daemonset/"$name" --timeout=5s > /dev/null; then
            echo "daemonset/$name is ready"
            return 0
        fi
        attempt=$((attempt + 1))
        if [[ $attempt -ge 36 ]]; then
            echo "Timed out waiting for daemonset/$name" >&2
            vm_kubectl -n "$ns" get ds,pods -o wide >&2
            return 1
        fi
        echo "Waiting for daemonset/$name rollout"
        sleep 5
    done
}

# get_service_address <service> [namespace]
# Prints the LoadBalancer address, or nothing if none is assigned yet.
function get_service_address {
    local name=$1
    local ns=${2:-default}
    vm_kubectl -n "$ns" get svc "$name" \
        -o "jsonpath='{.status.loadBalancer.ingress[0].ip}{.status.loadBalancer.ingress[0].hostname}'" \
        | tr -d "'" | tail -n 1
}

# wait_for_service_addresses <service>...
# Returns once every named service has a LoadBalancer address.
function wait_for_service_addresses {
    local attempt=0
    local svc addr pending

    while true; do
        pending=""
        for svc in "$@"; do
            addr=$(get_service_address "$svc")
            if [[ -z "$addr" ]]; then
                pending="$pending $svc"
            fi
        done
        if [[ -z "$pending" ]]; then
            for svc in "$@"; do
                echo "service/$svc external address: $(get_service_address "$svc")"
            done
            return 0
        fi
        attempt=$((attempt + 1))
        if [[ $attempt -ge 36 ]]; then
            echo "Timed out waiting for service addresses:$pending" >&2
            vm_kubectl get svc >&2
            return 1
        fi
        echo "Waiting for external address of:$pending"
        sleep 5
    done
}
