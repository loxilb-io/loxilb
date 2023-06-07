export LOXILB_IP=$(ip a |grep global | grep -v '10.0.2.15' | awk '{print $2}' | cut -f1 -d '/')

apt-get update
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu  $(lsb_release -cs)  stable"
apt-get update
apt-get install -y docker-ce
docker run -u root --cap-add SYS_ADMIN   --restart unless-stopped --privileged -dit -v /dev/log:/dev/log --net=host --name loxilb ghcr.io/loxilb-io/loxilb:latest
echo alias loxicmd=\"sudo docker exec -it loxilb loxicmd\" >> ~/.bashrc
echo alias loxilb=\"sudo docker exec -it loxilb \" >> ~/.bashrc

echo $LOXILB_IP > /vagrant/loxilb-ip
