source /vagrant/common.sh

function wait_k0s_cluster_ready {
    Res=$(sudo k0s kubectl get pods -A |
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

function wait_k0s_cluster_ready_full {
  i=1
  nr=0
  for ((;;)) do
    wait_k0s_cluster_ready
    nr=$?
    if [[ $nr == 0 ]]; then
        echo "Cluster is ready"
        break
    fi
    i=$(( $i + 1 ))
    if [[ $i -ge 40 ]]; then
        echo "Cluster is not ready.Giving up"
        sudo k0s kubectl get svc
        sudo k0s kubectl get pods -A
        exit 1
    fi
    echo "Cluster is not ready...."
    sleep 10
  done
}

export LOXILB_IP=$(ip a |grep global | grep -v '10.0.2.15' | grep -v '192.168.80' | awk '{print $2}' | cut -f1 -d '/')

apt-get update
apt-get install -y software-properties-common ethtool

echo "Start K0s installation"

curl -sSLf https://get.k0s.sh | sudo sh
k0s install controller --single
k0s start

sleep 30
k0s status

# Check kubectl works
k0s kubectl get pods -A

echo "End K0s installation"
sleep 60
wait_k0s_cluster_ready_full

k0s kubectl apply -f /vagrant/loxilb.yml
sleep 60
wait_k0s_cluster_ready_full

k0s kubectl apply -f /vagrant/kube-loxilb.yml
sleep 30
wait_k0s_cluster_ready_full

k0s kubectl apply -f /vagrant/tcp-svc-lb.yml

# Wait for cluster to be ready
wait_k0s_cluster_ready_full
