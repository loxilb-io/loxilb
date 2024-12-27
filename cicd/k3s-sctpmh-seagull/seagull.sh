docker run -u root --cap-add SYS_ADMIN -i -t --rm --privileged --detach --entrypoint /bin/bash --name seagull  ghcr.io/loxilb-io/seagull:ubuntu1804

ifconfig eth1 promisc
ifconfig eth2 promisc

docker network create -d macvlan -o parent=eth1 --subnet 4.0.5.0/24 --gateway 4.0.5.254 net1
docker network create -d macvlan -o parent=eth2 --subnet 4.0.4.0/24 --gateway 4.0.4.254 net2

docker network connect net1 seagull --ip=4.0.5.4
docker network connect net2 seagull --ip=4.0.4.4
docker exec -i seagull ifconfig eth0 0

