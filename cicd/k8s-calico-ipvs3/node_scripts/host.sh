# Install Bird to work with k3s
sudo apt install bird2 --yes

sleep 5

sudo cp -f /vagrant/bird_config/bird.conf /etc/bird/bird.conf
if [ ! -f  /var/log/bird.log ]; then
  sudo touch /var/log/bird.log
fi
sudo chown bird:bird /var/log/bird.log
sudo service bird start

echo "Host is up"
