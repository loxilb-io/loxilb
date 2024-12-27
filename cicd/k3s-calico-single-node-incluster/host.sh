sudo su
echo "123.123.123.1 k8s-svc" >> /etc/hosts
ifconfig eth2 mtu 1450
ip route add 123.123.123.0/24 via 192.168.90.10
echo "Host is up"
