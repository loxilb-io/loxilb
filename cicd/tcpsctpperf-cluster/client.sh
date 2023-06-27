export CLIENT_IP=$(ip a |grep global | grep -v '10.0.2.15' | grep '192.168.80' | awk '{print $2}' | cut -f1 -d '/')
echo $CLIENT_IP > /vagrant/client-ip
docker pull loxilbio/seastar-dev:latest
docker run -u root --cap-add SYS_ADMIN   --restart unless-stopped --privileged -dit -v /dev/log:/dev/log --net=host --entrypoint /bin/bash --name loxilbio/seastar-dev:latest
