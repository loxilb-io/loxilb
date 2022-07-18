# loxilb - How to build/run

## Right from code (difficult)

* Build custom iproute2 package 

```
git clone https://github.com/loxilb-io/iproute2.git
cd iproute2
cd libbpf/src/
mkdir build
DESTDIR=build make install
cd ../../
export PKG_CONFIG_PATH=$PKG_CONFIG_PATH:`pwd`/libbpf/src/
LIBBPF_FORCE=on LIBBPF_DIR=`pwd`/libbpf/src/build ./configure
make
sudo cp -f tc/tc /usr/local/sbin/ntc
```

* Build and run loxilb 

```
git clone https://github.com/loxilb-io/loxilb.git
cd loxilb
./ebpf/utils/mkllb_bpffs.sh
make
cd ebpf/libbpf/src
sudo make install
cd -
sudo ./loxilb 
```
* To run with integrated api-server, we can use the following :

```
./loxilb --tls-key=api/certification/server.key --tls-certificate=api/certification/server.crt --host=0.0.0.0 --port=11111 --tls-port=8091 -a
```

## From docker (easy)

* Get the loxilb official docker image 

```
docker pull loxilbio/loxilb:beta
```

* To run loxilb docker, we can use the following commands :

```
docker run -u root --cap-add SYS_ADMIN   --restart unless-stopped --privileged -dit -v /dev/log:/dev/log -v /var/run/:/var/run --name loxilb loxilbio/loxilb:beta
```

* To drop in to a shell of loxilb doker :

```
docker exec -it loxilb bash
```

* For load-balancing to effetively work in a bare-metal environment, we need multiple interfaces assigned to the docker (external and internal connectivitiy) 

  loxilb docker relies on docker's macvlan driver for achieving this. The following is an example of creating macvlan network and using with loxilb

```
# Create a mac-vlan (on an underlying interface enp0s3)
docker network create -d macvlan -o parent=enp0s3   --subnet 172.30.1.0/24   --gateway 172.30.1.254 --aux-address 'host=172.30.1.193’ llbnet

# Run loxilb docker with the created macvlan 
docker run -u root --cap-add SYS_ADMIN   --restart unless-stopped --privileged -dit -v /dev/log:/dev/log -v /var/run/:/var/run --net=llbnet --ip=172.30.1.193 --name loxilb loxilbio/loxilb:beta

# If we still want to connect loxilb docker additionally to docker's default network or more macvlan networks
docker network connect bridge loxilb
```
  *Note - While working with macvlan interfaces, the parent/underlying interface should be put in promiscous mode*

* Finally, to run loxilb docker with all modules loaded, the following command can be used :

```
docker run -u root --cap-add SYS_ADMIN   --restart unless-stopped --privileged -dit -v /dev/log:/dev/log -v /var/run/:/var/run --net=llbnet --ip=172.30.1.193 --entrypoint /root/loxilb-io/loxilb/loxilb --name loxilb loxilbio/loxilb:beta --tls-key=/root/loxilb-io/loxilb/api/certification/server.key --tls-certificate=/root/loxilb-io/loxilb/api/certification/server.crt --host=0.0.0.0 --port=11111 --tls-port=8091 -a
```

