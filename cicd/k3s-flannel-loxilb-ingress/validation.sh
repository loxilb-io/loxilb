#!/bin/bash
source ../common.sh
echo k3s-loxi-ingress

if [ "$1" ]; then
  KUBECONFIG="$1"
fi

# Set space as the delimiter
IFS=' '

sleep 45

echo "=== Debug: Pod Status ==="
vagrant ssh master -c 'sudo kubectl get pods -A -o wide'

echo "=== Debug: Service Events (loxilb-ingress-manager) ==="
vagrant ssh master -c 'sudo kubectl get endpoint loxilb-ingress-manager -n kube-system' 2>&1 | tail -30

echo "=== Debug: Ingress Events ==="
vagrant ssh master -c 'sudo kubectl describe ingress -A' 2>&1 | tail -30

echo "=== Debug: kube-loxilb logs ==="
vagrant ssh master -c 'sudo kubectl logs -n kube-system -l app=kube-loxilb-app --tail=80' 2>&1

echo "=== Debug: loxilb container logs ==="
vagrant ssh loxilb -c 'sudo docker logs loxilb --tail 80' 2>&1

echo "=== Debug: loxilb API health check ==="
vagrant ssh loxilb -c 'curl -s -o /dev/null -w "HTTP_CODE:%{http_code}" http://localhost:11111/netlox/v1/config/loadbalancer/all' 2>&1

echo "=== Debug: Network connectivity master -> loxilb ==="
vagrant ssh master -c 'ping -c 2 -W 2 192.168.80.9' 2>&1
vagrant ssh master -c 'curl -s -o /dev/null -w "HTTP_CODE:%{http_code}" --connect-timeout 5 http://192.168.80.9:11111/netlox/v1/config/loadbalancer/all' 2>&1

echo "Service Info"
vagrant ssh master -c 'sudo kubectl get svc -A'
echo "Ingress Info"
vagrant ssh master -c 'sudo kubectl get ingress -A'
echo "LB Info"
vagrant ssh loxilb -c 'sudo docker exec -i loxilb loxicmd get lb -o wide'
echo "EP Info"
vagrant ssh loxilb -c 'sudo docker exec -i loxilb loxicmd get ep -o wide'

print_debug_info() {
  echo "llb1 route-info"
  vagrant ssh loxilb -c 'ip route'
  vagrant ssh master -c 'sudo kubectl get pods -A -o wide'
  vagrant ssh master -c 'sudo kubectl get svc -A'
  vagrant ssh master -c 'sudo kubectl get nodes -o wide'
  echo "=== Failed Debug: kube-loxilb logs (full) ==="
  vagrant ssh master -c 'sudo kubectl logs -n kube-system -l app=kube-loxilb-app --tail=200' 2>&1
  echo "=== Failed Debug: loxilb container logs (full) ==="
  vagrant ssh loxilb -c 'sudo docker logs loxilb --tail 200' 2>&1
  echo "=== Failed Debug: Service describe ==="
  vagrant ssh master -c 'sudo kubectl describe svc loxilb-ingress-manager -n kube-system' 2>&1
  vagrant ssh master -c 'sudo kubectl get endpoint loxilb-ingress-manager -n kube-system' 2>&1

  echo "=== Failed Debug: Ingress describe ==="
  vagrant ssh master -c 'sudo kubectl describe ingress -A' 2>&1
}

out=$(curl -s --connect-timeout 30 -H "Application/json" -H "Content-type: application/json" -H "HOST: loxilb.io" --insecure https://192.168.80.9:443)
if [[ ${out} == *"Welcome to nginx"* ]]; then
  echo "k3s-loxi-ingress tcp [OK]"
else
  echo "k3s-loxi-ingress tcp [FAILED]"
  print_debug_info
  exit 1
fi

exit
