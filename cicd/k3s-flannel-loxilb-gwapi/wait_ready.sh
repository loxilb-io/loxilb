#!/bin/bash

function wait_cluster_ready_full {
  sudo kubectl wait pod --all --for=condition=Ready --namespace=kube-system --timeout=240s
  sudo kubectl wait pod --all --for=condition=Ready --namespace=default --timeout=60s
}

wait_cluster_ready_full
