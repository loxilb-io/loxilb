export LOXILB_IP=$(ip a |grep global | grep -v '10.0.2.15' | grep -v '192.168.80' | awk '{print $2}' | cut -f1 -d '/')

apt-get update
apt-get install -y software-properties-common
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
add-apt-repository -y "deb [arch=amd64] https://download.docker.com/linux/ubuntu  $(lsb_release -cs)  stable"
apt-get update
apt-get install -y docker-ce

mkdir cert
cd cert
wget --retry-connrefused --waitretry=1 --read-timeout=20 --timeout=15 -t 3  https://github.com/FiloSottile/mkcert/releases/download/v1.4.3/mkcert-v1.4.3-linux-amd64
chmod +x mkcert-v1.4.3-linux-amd64
mv mkcert-v1.4.3-linux-amd64 mkcert
mkdir loxilb.io
export CAROOT=`pwd`/loxilb
./mkcert -install
./mkcert 192.168.80.9
cp loxilb/rootCA.pem ./rootCA.crt
cp loxilb/rootCA.pem /vagrant/loxilbCA.pem
mv 192.168.80.9.pem ./server.crt
mv 192.168.80.9-key.pem ./server.key
cd -

docker run -u root --cap-add SYS_ADMIN   --restart unless-stopped --privileged -dit -v /dev/log:/dev/log -v `pwd`/cert:/opt/loxilb/cert/ --net=host --name loxilb ghcr.io/loxilb-io/loxilb:latest --tls
echo alias loxicmd=\"sudo docker exec -it loxilb loxicmd\" >> ~/.bashrc
echo alias loxilb=\"sudo docker exec -it loxilb \" >> ~/.bashrc

echo $LOXILB_IP > /vagrant/loxilb-ip
