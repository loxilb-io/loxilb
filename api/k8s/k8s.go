/*
 * Copyright (c) 2023 NetLOX Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at:
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package k8s

import (
	"errors"
	"net"
	"os"

	cmn "github.com/loxilb-io/loxilb/common"
	tk "github.com/loxilb-io/loxilib"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	config   *rest.Config
	ApiHooks cmn.NetHookInterface
	stopCh   chan struct{}
)

func K8sApiInit(k8sConfigFile string, hooks cmn.NetHookInterface) error {

	var err error
	nodeIP := os.Getenv("MY_NODE_IP")
	if net.ParseIP(nodeIP) == nil {
		tk.LogIt(tk.LogError, "NodeIP(%s) not found\n", nodeIP)
		os.Exit(1)
	}

	tk.LogIt(tk.LogDebug, "K8s NodeIP(%s)\n", nodeIP)

	if k8sConfigFile != "cluster" {
		config, err = clientcmd.BuildConfigFromFlags("", k8sConfigFile)
		if err != nil {
			tk.LogIt(tk.LogError, "Config(%s) build failed:%s\n", k8sConfigFile, err)
			return err
		}
	} else {
		config, err = rest.InClusterConfig()
		if err != nil {
			tk.LogIt(tk.LogError, "InClusterConfig build failed:%s\n", err)
			return err
		}
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		tk.LogIt(tk.LogError, "NewForConfig failed:%s\n", err)
		return err
	}

	watchlist := cache.NewListWatchFromClient(
		clientset.CoreV1().RESTClient(),
		string(v1.ResourcePods),
		v1.NamespaceAll,
		fields.Everything(),
	)
	_, controller := cache.NewInformer(
		watchlist,
		&v1.Pod{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				switch pod := obj.(type) {
				case *v1.Pod:
					tk.LogIt(tk.LogInfo, "Pod(%s) add: %s - %s:\n", pod.Name, pod.Status.PodIP, pod.Status.HostIP)
					if pod.Status.HostIP == nodeIP && pod.Status.PodIP != nodeIP && pod.Status.PodIP != "" {
						_, err := ApiHooks.NetAddrAdd(&cmn.IPAddrMod{Dev: pod.Name, IP: pod.Status.PodIP + "/32"})
						if err != nil {
							tk.LogIt(tk.LogDebug, "Pod(%s) add: %s - %s: failed - %s\n", pod.Name, pod.Status.PodIP, pod.Status.HostIP, err)
						} else {
							tk.LogIt(tk.LogDebug, "Pod(%s) added: %s - %s \n", pod.Name, pod.Status.PodIP, pod.Status.HostIP)
						}
					}
				}
			},
			DeleteFunc: func(obj interface{}) {
				switch pod := obj.(type) {
				case *v1.Pod:
					tk.LogIt(tk.LogInfo, "Pod(%s) delete: %s - %s: \n", pod.Name, pod.Status.PodIP, pod.Status.HostIP)
					if pod.Status.HostIP == nodeIP && pod.Status.PodIP != nodeIP && pod.Status.PodIP != "" {
						_, err := ApiHooks.NetAddrDel(&cmn.IPAddrMod{Dev: pod.Name, IP: pod.Status.PodIP + "/32"})
						if err != nil {
							tk.LogIt(tk.LogDebug, "Pod(%s) delete: %s - %s: failed - %s\n", pod.Name, pod.Status.PodIP, pod.Status.HostIP, err)
						} else {
							tk.LogIt(tk.LogDebug, "Pod(%s) deleted: %s - %s \n", pod.Name, pod.Status.PodIP, pod.Status.HostIP)
						}
					}
				}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				switch oldPod := oldObj.(type) {
				case *v1.Pod:
					if oldPod.Status.HostIP == nodeIP && oldPod.Status.PodIP != nodeIP && oldPod.Status.PodIP != "" {
						_, err := ApiHooks.NetAddrDel(&cmn.IPAddrMod{Dev: oldPod.Name, IP: oldPod.Status.PodIP + "/32"})
						if err != nil {
							tk.LogIt(tk.LogDebug, "Pod(%s) delete: %s - %s: failed - %s\n", oldPod.Name, oldPod.Status.PodIP, oldPod.Status.HostIP, err)
						} else {
							tk.LogIt(tk.LogDebug, "Pod(%s) deleted: %s - %s \n", oldPod.Name, oldPod.Status.PodIP, oldPod.Status.HostIP)
						}
					}
				}
				switch pod := newObj.(type) {
				case *v1.Pod:
					tk.LogIt(tk.LogInfo, "Pod(%s) modify: %s - %s:\n", pod.Name, pod.Status.PodIP, pod.Status.HostIP)
					if pod.Status.HostIP == nodeIP && pod.Status.PodIP != nodeIP && pod.Status.PodIP != "" {
						_, err := ApiHooks.NetAddrAdd(&cmn.IPAddrMod{Dev: pod.Name, IP: pod.Status.PodIP + "/32"})
						if err != nil {
							tk.LogIt(tk.LogDebug, "Pod(%s) modify: %s - %s: failed - %s\n", pod.Name, pod.Status.PodIP, pod.Status.HostIP, err)
						} else {
							tk.LogIt(tk.LogDebug, "Pod(%s) modified: %s - %s \n", pod.Name, pod.Status.PodIP, pod.Status.HostIP)
						}
					}
				}
			},
		},
	)

	if controller != nil {
		ApiHooks = hooks
		stopCh = make(chan struct{})
		go controller.Run(stopCh)
		tk.LogIt(tk.LogInfo, "K8s API Init done\n")
		return nil

	}
	return errors.New("k8s api init failed")
}

func K8sApiClose() {
	close(stopCh)
}
