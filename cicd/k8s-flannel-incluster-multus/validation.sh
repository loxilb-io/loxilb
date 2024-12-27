#!/bin/bash
source ../common.sh
echo k8s-flannel-incluster

if [ "$1" ]; then
  KUBECONFIG="$1"
fi

echo -e "\nEnd Points List"
echo "******************************************************************************"
vagrant ssh master -c 'kubectl get endpoints -A' 2> /dev/null
echo "******************************************************************************"
echo -e "\nSVC List"
echo "******************************************************************************"
vagrant ssh master -c 'kubectl get svc' 2> /dev/null
echo "******************************************************************************"
echo -e "\nPod List"
echo "******************************************************************************"
vagrant ssh master -c 'kubectl get pods -A' 2> /dev/null

out=$(vagrant ssh host -c "curl -s --connect-timeout 10 http://123.123.123.205:55002" 2> /dev/null)
#echo $out
if [[ ${out} == *"nginx"* ]]; then
  echo -e "k8s-flannel-incluster TCP\t[OK]"
else
  echo -e "k8s-flannel-incluster TCP\t[FAILED]"
  code=1
fi

exit $code
