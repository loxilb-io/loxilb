#!/bin/bash
# Tear down everything config.sh created.
cd "$(dirname "$0")"
CLUSTER=${CLUSTER:-kv}
SECNET=${SECNET:-secnet}
docker rm -f client >/dev/null 2>&1 || true
if kind get clusters 2>/dev/null | grep -qx "$CLUSTER"; then
  kind delete cluster --name "$CLUSTER"
fi
docker network rm "$SECNET" >/dev/null 2>&1 || true
echo "kind-kubevirt-multus: cleaned up (cluster=$CLUSTER network=$SECNET client)"
