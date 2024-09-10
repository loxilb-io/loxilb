/*
 * Copyright (c) 2024 NetLOX Inc
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

package loxinet

import (
	"net"
)

// CloudHookInterface - Go interface which needs to be implemented to
type CloudHookInterface interface {
	CloudAPIInit(cloudCIDRBlock string) error
	CloudPrepareVIPNetWork() error
	CloudUnPrepareVIPNetWork() error
	CloudDestroyVIPNetWork() error
	CloudUpdatePrivateIP(vIP net.IP, eIP net.IP, add bool) error
}

func CloudHookNew(cloudLabel string) CloudHookInterface {
	if mh.cloudLabel == "aws" {
		return AWSCloudHookNew()
	}
	return nil
}
