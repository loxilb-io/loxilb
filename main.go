/*
 * Copyright (c) 2022 NetLOX Inc
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

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/jessevdk/go-flags"
	opts "github.com/loxilb-io/loxilb/options"
	ln "github.com/loxilb-io/loxilb/pkg/loxinet"
)

var version string = "0.9.6-beta"
var buildInfo string = ""

func main() {
	fmt.Printf("loxilb start\n")
	fmt.Printf("Build Information: %s\n", buildInfo) // 빌드 정보 출력

	// Parse command-line arguments
	_, err := flags.Parse(&opts.Opts)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if opts.Opts.Version {
		fmt.Printf("loxilb version: %s %s\n", version, buildInfo)
		os.Exit(0)
	}

	go ln.LoxiXsyncMain(opts.Opts.RPC)
	// Need some time for RPC Handler to be up
	time.Sleep(2 * time.Second)

	ln.Main()
}
