# Install Bird to work with k3s
sudo apt-get update
sudo apt-get -y install bird2 lksctp-tools

sleep 5

sudo cp -f /vagrant/bird_config/bird.conf /etc/bird/bird.conf
if [ ! -f  /var/log/bird.log ]; then
  sudo touch /var/log/bird.log
fi
sudo chown bird:bird /var/log/bird.log
sudo service bird restart

echo "Host is up"
