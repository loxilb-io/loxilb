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
    "os/exec"
    "github.com/jessevdk/go-flags"
    opts "loxilb/options"
    ln "loxilb/loxinet"
)

const (
    MKFS_SCRIPT = "/usr/local/sbin/mkllb_bpffs"
    RUNNING_FLAG_FILE = "/var/run/loxilb"
    BPF_FS_CHK_FILE = "/opt/loxilb/dp/bpf/intf_map"
)

func fileExists(fname string) bool {
    info, err := os.Stat(fname)
    if os.IsNotExist(err) {
       return false
    }
    return !info.IsDir()
}

func fileCreate(fname string) int {
    file, e := os.Create(fname)
    if e != nil {
       return -1
    } 
    file.Close()
    return 0
}

var version string = "0.0.1"

func main() {
    fmt.Printf("Start\n")

    _, err := flags.Parse(&opts.Opts)
    if err != nil {
        fmt.Println(err)
        os.Exit(1)
    }

    if opts.Opts.Version {
        fmt.Printf("loxilb version: %s\n", version)
        os.Exit(0)
    }

    if fileExists(BPF_FS_CHK_FILE) == false {
        if fileExists(MKFS_SCRIPT) {
            _, err := exec.Command("/bin/bash", MKFS_SCRIPT).Output()
            if err != nil {
                fmt.Println(err)
                os.Exit(1)
            }
        }
    }

    ln.LoxiNetMain()
}
