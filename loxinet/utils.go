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

package loxinet

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	tk "github.com/loxilb-io/loxilib"
	nlp "github.com/vishvananda/netlink"
)

// IterIntf - interface implementation to iterate various loxinet
// subsystems entitities
type IterIntf interface {
	NodeWalker(b string)
}

// FileExists - Check if file exists
func FileExists(fname string) bool {
	info, err := os.Stat(fname)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// FileCreate - Create a file
func FileCreate(fname string) int {
	file, e := os.Create(fname)
	if e != nil {
		return -1
	}
	file.Close()
	return 0
}

// IsLoxiAPIActive - Check if API url is active
func IsLoxiAPIActive(url string) bool {
	timeout := time.Duration(1 * time.Second)
	client := http.Client{Timeout: timeout}
	_, e := client.Get(url)
	return e == nil
}

// ReadPIDFile - Read a PID file
func ReadPIDFile(pf string) int {

	if exists := FileExists(pf); !exists {
		return 0
	}

	d, err := ioutil.ReadFile(pf)
	if err != nil {
		return 0
	}

	pid, err := strconv.Atoi(string(bytes.TrimSpace(d)))
	if err != nil {
		return 0
	}

	p, err1 := os.FindProcess(int(pid))
	if err1 != nil {
		return 0
	}

	err = p.Signal(syscall.Signal(0))
	if err != nil {
		return 0
	}

	return pid
}

// RunCommand - Run a bash command
func RunCommand(command string, isFatal bool) (int, error) {
	cmd := exec.Command("bash", "-c", command)
	err := cmd.Run()
	if err != nil {
		tk.LogIt(tk.LogError, "Error in running %s:%s\n", command, err)
		if isFatal {
			os.Exit(1)
		}
		return 0, err
	}

	return 0, nil
}

// LogString2Level - Convert log level in string to LogLevelT
func LogString2Level(logStr string) tk.LogLevelT {
	logLevel := tk.LogDebug
	switch logStr {
	case "info":
		logLevel = tk.LogInfo
	case "error":
		logLevel = tk.LogError
	case "notice":
		logLevel = tk.LogNotice
	case "warning":
		logLevel = tk.LogWarning
	case "alert":
		logLevel = tk.LogAlert
	case "critical":
		logLevel = tk.LogCritical
	case "emergency":
		logLevel = tk.LogEmerg
	default:
		logLevel = tk.LogDebug
	}
	return logLevel
}

// KAString2Mode - Convert ka mode in string opts to spawn/KAMode
func KAString2Mode(kaStr string) CIKAArgs {
	spawnKa := false
	interval := int64(0)
	sourceIP := net.ParseIP("0.0.0.0")

	if kaStr == "none" {
		return CIKAArgs{SpawnKa: spawnKa, RemoteIP: nil, Interval: interval}
	}

	kaArgs := strings.Split(kaStr, ":")

	remote := net.ParseIP(kaArgs[0])
	if remote == nil {
		return CIKAArgs{SpawnKa: spawnKa, RemoteIP: nil, SourceIP: nil, Interval: interval}
	}

	if len(kaArgs) > 1 {
		sourceIP = net.ParseIP(kaArgs[1])
	}

	if len(kaArgs) > 2 {
		interval, _ = strconv.ParseInt(kaArgs[2], 10, 32)
	}
	spawnKa = true
	return CIKAArgs{SpawnKa: spawnKa, RemoteIP: remote, SourceIP: sourceIP, Interval: interval}

}

// HTTPSProber - Do a https probe for given url
// returns true/false depending on whether probing was successful
func HTTPSProber(urls string, cert tls.Certificate, certPool *x509.CertPool, resp string) bool {
	var err error
	var req *http.Request
	var res *http.Response

	timeout := time.Duration(2 * time.Second)
	client := http.Client{Timeout: timeout,
		Transport: &http.Transport{
			IdleConnTimeout: 5 * time.Second,
			TLSClientConfig: &tls.Config{Certificates: []tls.Certificate{cert},
				RootCAs: certPool}},
	}
	if req, err = http.NewRequest(http.MethodGet, urls, nil); err != nil {
		tk.LogIt(tk.LogError, "unable to create http request: %s\n", err)
		return false
	}

	res, err = client.Do(req)
	if err != nil || res.StatusCode != 200 {
		tk.LogIt(tk.LogError, "unable to create http request: %s\n", err)
		return false
	}
	defer res.Body.Close()
	if resp != "" {
		data, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return false
		}
		return string(data) == resp
	}

	return true
}

// IsIPHostAddr - Check if provided address is a local address
func IsIPHostAddr(ipString string) bool {
	// get list of available addresses
	addr, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}

	for _, addr := range addr {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			// check if IPv4 or IPv6 is not nil
			if ipnet.IP.To4() != nil || ipnet.IP.To16() != nil {
				if ipnet.IP.String() == ipString {
					return true
				}
			}
		}
	}

	return false
}

// IsIPHostNetAddr - Check if provided address is a local subnet
func IsIPHostNetAddr(ip net.IP) bool {
	// get list of available addresses
	addr, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}

	for _, addr := range addr {
		if ipnet, ok := addr.(*net.IPNet); ok {
			// check if IPv4 or IPv6 is not nil
			if ipnet.IP.To4() != nil || ipnet.IP.To16() != nil {
				if ipnet.Contains(ip) {
					return true
				}
			}
		}
	}

	return false
}

// GratArpReq - sends a gratuitious arp reply given the DIP, SIP and interface name
func GratArpReq(AdvIP net.IP, ifName string) (int, error) {
	bcAddr := []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
	fd, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_DGRAM, int(tk.Htons(syscall.ETH_P_ARP)))
	if err != nil {
		return -1, errors.New("af-packet-err")
	}
	defer syscall.Close(fd)

	if err := syscall.BindToDevice(fd, ifName); err != nil {
		return -1, errors.New("bind-err")
	}

	ifi, err := net.InterfaceByName(ifName)
	if err != nil {
		return -1, errors.New("intf-err")
	}

	ll := syscall.SockaddrLinklayer{
		Protocol: tk.Htons(syscall.ETH_P_ARP),
		Ifindex:  ifi.Index,
		Pkttype:  0, // syscall.PACKET_HOST
		Hatype:   1,
		Halen:    6,
	}

	for i := 0; i < 8; i++ {
		ll.Addr[i] = 0xff
	}

	buf := new(bytes.Buffer)

	var sb = make([]byte, 2)
	binary.BigEndian.PutUint16(sb, 1) // HwType = 1
	buf.Write(sb)

	binary.BigEndian.PutUint16(sb, 0x0800) // protoType
	buf.Write(sb)

	buf.Write([]byte{6}) // hwAddrLen
	buf.Write([]byte{4}) // protoAddrLen

	binary.BigEndian.PutUint16(sb, 0x2) // OpCode
	buf.Write(sb)

	buf.Write(ifi.HardwareAddr) // senderHwAddr
	buf.Write(AdvIP.To4())      // senderProtoAddr

	buf.Write(bcAddr)      // targetHwAddr
	buf.Write(AdvIP.To4()) // targetProtoAddr

	if err := syscall.Bind(fd, &ll); err != nil {
		return -1, errors.New("bind-err")
	}
	if err := syscall.Sendto(fd, buf.Bytes(), 0, &ll); err != nil {
		return -1, errors.New("send-err")
	}

	return 0, nil
}

// GratArpReq - sends a gratuitious arp reply given the DIP, SIP and interface name
func GratArpReqWithCtx(ctx context.Context, rCh chan<- int, AdvIP net.IP, ifName string) (int, error) {
	for {
		select {
		case <-ctx.Done():
			return -1, ctx.Err()
		default:
			ret, _ := GratArpReq(AdvIP, ifName)
			rCh <- ret
			return 0, nil
		}
	}
}

func FormatTimedelta(t time.Time) string {
	d := time.Now().Unix() - t.Unix()
	u := uint64(d)
	neg := d < 0
	if neg {
		u = -u
	}
	secs := u % 60
	u /= 60
	mins := u % 60
	u /= 60
	hours := u % 24
	days := u / 24

	if days == 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hours, mins, secs)
	}
	return fmt.Sprintf("%dd ", days) + fmt.Sprintf("%02d:%02d:%02d", hours, mins, secs)
}

// Ntohll - Network to host byte-order long long
func Ntohll(i uint64) uint64 {
	return binary.BigEndian.Uint64((*(*[8]byte)(unsafe.Pointer(&i)))[:])
}

// GetIfaceIpAddr - Get interface IP address
func GetIfaceIpAddr(ifName string) (addr net.IP, err error) {
	var (
		ief    *net.Interface
		addrs  []net.Addr
		ipAddr net.IP
	)
	if ief, err = net.InterfaceByName(ifName); err != nil {
		return
	}
	if addrs, err = ief.Addrs(); err != nil {
		return
	}
	for _, addr := range addrs {
		if ipAddr = addr.(*net.IPNet).IP.To4(); ipAddr != nil {
			break
		}
	}
	if ipAddr == nil {
		return nil, errors.New(fmt.Sprintf("%s - no ipv4 address\n", ifName))
	}
	return ipAddr, nil
}

// SendArpReq - sends a  arp request given the DIP, SIP and interface name
func SendArpReq(AdvIP net.IP, ifName string) (int, error) {
	zeroAddr := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

	srcIP, err := GetIfaceIpAddr(ifName)
	if err != nil {
		return -1, errors.New(fmt.Sprintf("%s", err))
	}
	fd, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_DGRAM, int(tk.Htons(syscall.ETH_P_ARP)))
	if err != nil {
		return -1, errors.New("af-packet-err")
	}
	defer syscall.Close(fd)

	if err := syscall.BindToDevice(fd, ifName); err != nil {
		return -1, errors.New("bind-err")
	}

	ifi, err := net.InterfaceByName(ifName)
	if err != nil {
		return -1, errors.New("intf-err")
	}

	ll := syscall.SockaddrLinklayer{
		Protocol: tk.Htons(syscall.ETH_P_ARP),
		Ifindex:  ifi.Index,
		Pkttype:  0, // syscall.PACKET_HOST
		Hatype:   1,
		Halen:    6,
	}

	for i := 0; i < 8; i++ {
		ll.Addr[i] = 0xff
	}

	buf := new(bytes.Buffer)

	var sb = make([]byte, 2)
	binary.BigEndian.PutUint16(sb, 1) // HwType = 1
	buf.Write(sb)

	binary.BigEndian.PutUint16(sb, 0x0800) // protoType
	buf.Write(sb)

	buf.Write([]byte{6}) // hwAddrLen
	buf.Write([]byte{4}) // protoAddrLen

	binary.BigEndian.PutUint16(sb, 0x1) // OpCode
	buf.Write(sb)

	buf.Write(ifi.HardwareAddr) // senderHwAddr
	buf.Write(srcIP.To4())      // senderProtoAddr

	buf.Write(zeroAddr)    // targetHwAddr
	buf.Write(AdvIP.To4()) // targetProtoAddr

	if err := syscall.Bind(fd, &ll); err != nil {
		return -1, errors.New("bind-err")
	}
	if err := syscall.Sendto(fd, buf.Bytes(), 0, &ll); err != nil {
		return -1, errors.New("send-err")
	}

	return 0, nil
}

// ArpReqWithCtx - sends a arp req given the DIP, SIP and interface name
func ArpReqWithCtx(ctx context.Context, rCh chan<- int, AdvIP net.IP, ifName string) (int, error) {
	for {
		select {
		case <-ctx.Done():
			return -1, ctx.Err()
		default:
			ret, _ := SendArpReq(AdvIP, ifName)
			rCh <- ret
			return 0, nil
		}
	}
}

func ArpResolver(dIP uint32) {
	var gw net.IP
	var ifName string
	dest := tk.NltoIP(dIP)

	routes, err := nlp.RouteGet(dest)
	if err != nil {
		return
	}

	for _, r := range routes {
		if r.Gw == nil {
			gw = r.Dst.IP
		} else {
			gw = r.Gw
		}
		if gw == nil {
			continue
		}
		link, err := nlp.LinkByIndex(r.LinkIndex)
		if err != nil {
			return
		}
		ifName = link.Attrs().Name
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		rCh := make(chan int)
		go ArpReqWithCtx(ctx, rCh, gw, ifName)
		select {
		case <-rCh:
			break
		case <-ctx.Done():
			tk.LogIt(tk.LogInfo, "%s - iface %s : ARP timeout\n", gw.String(), ifName)
		}
		return
	}
}
