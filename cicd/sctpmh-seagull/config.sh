#!/bin/bash
vagrant destroy -f
vagrant up

for i in {1..60}
do
    ping 4.0.4.3 -c 1 -W 1 2>&1> /dev/null;
    if [[ $? -eq 0 ]]
    then
     echo -e "Machine rebooted [OK]"
     code=0
     break
    else
        echo -e "Waiting for machine to be UP"
        sleep 1
    fi
done
if [[ $code == 0 ]];
then
    vagrant ssh bastion -c 'sudo /vagrant/setup.sh'
else
    echo "VM not up"
fi
