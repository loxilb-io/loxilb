#!/bin/bash

wait_for_daemonset() {
	local name=$1
	local attempt=0

	while true; do
		if vagrant ssh master1 -c "sudo kubectl rollout status daemonset/${name} --timeout=5s" > /dev/null 2>&1; then
			echo "daemonset/${name} is ready"
			return 0
		fi

		attempt=$((attempt + 1))
		if [[ ${attempt} -ge 36 ]]; then
			echo "Timed out waiting for daemonset/${name}" >&2
			return 1
		fi

		echo "Waiting for daemonset/${name} rollout"
		sleep 5
	done
}

wait_for_service_ip() {
	local name=$1
	local attempt=0
	local lb_ip
	local svc_output
    local pod_output

	while true; do
		lb_ip=$(vagrant ssh master1 -c "sudo kubectl get svc ${name} -o jsonpath='{.status.loadBalancer.ingress[0].ip}'" 2> /dev/null | tr -d '\r' | tail -n 1)
		if [[ -n "${lb_ip}" ]]; then
			echo "service/${name} external IP: ${lb_ip}"
			return 0
		fi

		attempt=$((attempt + 1))
		if [[ ${attempt} -ge 36 ]]; then
			echo "Timed out waiting for service/${name} external IP" >&2
			return 1
		fi

		svc_output=$(vagrant ssh master1 -c "sudo kubectl get svc ${name} -o wide" 2> /dev/null | tr -d '\r')
        pod_output=$(vagrant ssh master1 -c "sudo kubectl get pods -A" 2> /dev/null | tr -d '\r')
		echo "Waiting for service/${name} external IP"
		if [[ -n "${svc_output}" ]]; then
			echo "${svc_output}"
		fi
        if [[ -n "${pod_output}" ]]; then
            echo "${pod_output}"
        fi
		sleep 5
	done
}

vagrant global-status  | grep -i virtualbox | cut -f 1 -d ' ' | xargs -L 1 vagrant destroy -f
vagrant up
#sudo ip route add 123.123.123.1 via 192.168.90.10 || true
vagrant ssh master1 -c 'sudo kubectl create -f /vagrant/tcp-onearm-ds.yml'
vagrant ssh master1 -c 'sudo kubectl create -f /vagrant/udp-onearm-ds.yml'
vagrant ssh master1 -c 'sudo kubectl create -f /vagrant/sctp-onearm-ds.yml'

wait_for_daemonset tcp-onearm-ds
wait_for_daemonset udp-onearm-ds
wait_for_daemonset sctp-onearm-ds

wait_for_service_ip tcp-onearm-svc
wait_for_service_ip udp-onearm-svc
wait_for_service_ip sctp-onearm-svc
