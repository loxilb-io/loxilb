wait_for_file() {
	local file_path=$1
	local attempt=0

	while [[ ! -s "$file_path" ]]; do
		attempt=$((attempt + 1))
		if [[ $attempt -ge 60 ]]; then
			echo "Timed out waiting for $file_path" >&2
			exit 1
		fi
		sleep 2
	done
}

verify_apiserver_san() {
	local cert_path=/var/lib/rancher/k3s/server/tls/serving-kube-apiserver.crt

	if ! openssl x509 -in "$cert_path" -text | grep -q 'IP Address:192.168.80.80'; then
		echo "API server certificate does not include VIP 192.168.80.80 in SAN" >&2
		openssl x509 -in "$cert_path" -text | sed -n '/Subject Alternative Name/,+1p' >&2
		exit 1
	fi
}

sudo su
mkdir -p /etc/rancher/k3s
cat <<'EOF' > /etc/rancher/k3s/config.yaml
cluster-init: true
node-ip: 192.168.80.10
node-external-ip: 192.168.80.10
disable:
  - servicelb
  - traefik
disable-cloud-controller: true
flannel-iface: eth1
node-name: master1
tls-san:
  - 192.168.80.80
EOF
export MASTER_IP=$(ip a |grep global | grep -v '10.0.2.15' | grep -v '192.168.90' | grep '192.168.80' | awk '{print $2}' | cut -f1 -d '/')
curl -fL https://get.k3s.io | sh -s - server
curl -sfL https://github.com/loxilb-io/loxilb-ebpf/raw/main/kprobe/install.sh | sh -
wait_for_file /var/lib/rancher/k3s/server/node-token
wait_for_file /etc/rancher/k3s/k3s.yaml
wait_for_file /var/lib/rancher/k3s/server/tls/serving-kube-apiserver.crt
verify_apiserver_san
echo $MASTER_IP > /vagrant/master-ip
cp /var/lib/rancher/k3s/server/node-token /vagrant/node-token
cp /etc/rancher/k3s/k3s.yaml /vagrant/k3s.yaml
sed -i -e "s/127.0.0.1/192.168.80.80/g" /vagrant/k3s.yaml
sudo mkdir -p /etc/loxilb
sudo cp /vagrant/lbconfig.txt /etc/loxilb/
sudo cp /vagrant/EPconfig.txt /etc/loxilb/
