#!/bin/bash
set -euo pipefail

wait_for_cluster() {
	vagrant ssh master1 -c 'sudo /vagrant/wait_ready.sh'
}

build_stage_two_machines() {
	local workers=${WORKERS:-2}
	local machine_names=(master2 master3)
	local node_number

	for ((node_number = 1; node_number <= workers; node_number++)); do
		machine_names+=("worker${node_number}")
	done

	echo "${machine_names[@]}"
}

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

wait_for_service_addresses() {
	local services=("$@")
	local attempt=0
	local svc_name
	local lb_address
	local svc_output
	local pod_output
	local pending_services=()
	local pending_count=0
	local -A ready_services=()

	while true; do
		pending_services=()
		pending_count=0

		for svc_name in "${services[@]}"; do
			lb_address=$(vagrant ssh master1 -c "sudo kubectl get svc ${svc_name} -o jsonpath='{.status.loadBalancer.ingress[0].ip}{.status.loadBalancer.ingress[0].hostname}'" 2> /dev/null | tr -d '\r' | tail -n 1)
			if [[ -n "${lb_address}" ]]; then
				if [[ -z "${ready_services[$svc_name]:-}" ]]; then
					echo "service/${svc_name} external address: ${lb_address}"
					ready_services[$svc_name]="${lb_address}"
				fi
				continue
			fi

			pending_services+=("${svc_name}")
			pending_count=$((pending_count + 1))
			svc_output=$(vagrant ssh master1 -c "sudo kubectl get svc ${svc_name} -o wide" 2> /dev/null | tr -d '\r')
			if [[ -n "${svc_output}" ]]; then
				echo "Waiting for service/${svc_name} external address"
				echo "${svc_output}"
			fi
		done

		if [[ ${pending_count} -eq 0 ]]; then
			return 0
		fi

		attempt=$((attempt + 1))
		if [[ ${attempt} -ge 36 ]]; then
			echo "Timed out waiting for service addresses: ${pending_services[*]}" >&2
			return 1
		fi

		pod_output=$(vagrant ssh master1 -c "sudo kubectl get pods -A" 2> /dev/null | tr -d '\r')
		if [[ -n "${pod_output}" ]]; then
			echo "${pod_output}"
		fi
		sleep 5
	done
}

vagrant destroy -f || true
read -r -a stage_two_machines <<< "$(build_stage_two_machines)"

echo "Starting bootstrap machines in parallel"
vagrant up --parallel host master1

echo "Waiting for bootstrap cluster readiness"
wait_for_cluster

echo "Starting remaining machines in parallel: ${stage_two_machines[*]}"
vagrant up --parallel "${stage_two_machines[@]}"

echo "Waiting for full cluster readiness"
wait_for_cluster
#sudo ip route add 123.123.123.1 via 192.168.90.10 || true
vagrant ssh master1 -c 'sudo kubectl create -f /vagrant/tcp-onearm-ds.yml'
vagrant ssh master1 -c 'sudo kubectl create -f /vagrant/udp-onearm-ds.yml'
vagrant ssh master1 -c 'sudo kubectl create -f /vagrant/sctp-onearm-ds.yml'

wait_for_daemonset tcp-onearm-ds
wait_for_daemonset udp-onearm-ds
wait_for_daemonset sctp-onearm-ds

wait_for_service_addresses tcp-onearm-svc udp-onearm-svc sctp-onearm-svc
