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

package utils

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	tk "github.com/loxilb-io/loxilib"
	nlp "github.com/vishvananda/netlink"
)

const (
	ICMPv6NeighborAdvertisement = 136 // ICMPv6 Type for Neighbor Advertisement
)

type icmpv6Header struct {
	Type     uint8
	Code     uint8
	Checksum uint16
	Reserved uint32
}

func NetAdvertiseVI64Req(targetIP net.IP, ifName string) (int, error) {
	if targetIP == nil || ifName == "" || ifName == "lo" {
		return -1, errors.New("invalid parameters")
	}

	if !tk.IsNetIPv6(targetIP.String()) {
		return -1, errors.New("invalid parameters")
	}

	ifi, err := net.InterfaceByName(ifName)
	if err != nil {
		return -1, errors.New("intfv6-err")
	}

	srcIP := net.IPv6linklocalallnodes
	dstIP := net.IPv6linklocalallnodes

	// Create an ICMPv6 header for Neighbor Advertisement
	icmpHeader := &icmpv6Header{
		Type:     ICMPv6NeighborAdvertisement,
		Code:     0,
		Checksum: 0, // To be calculated later
		Reserved: 0,
	}

	payload := newNeighborAdvertisementPayload(targetIP, ifi.HardwareAddr)
	icmpData := append(icmpHeader.Marshal(), payload...)

	// Calculate checksum
	icmpHeader.Checksum = calculateChecksum(icmpData, srcIP, dstIP)
	icmpData = append(icmpHeader.Marshal(), payload...)

	fd, err := syscall.Socket(syscall.AF_INET6, syscall.SOCK_RAW, syscall.IPPROTO_ICMPV6)
	if err != nil {
		return -1, err
	}
	defer syscall.Close(fd)

	if err := syscall.BindToDevice(fd, ifName); err != nil {
		return -1, errors.New("bindv6-err")
	}

	dstAddr := &syscall.SockaddrInet6{
		Port: 0,
		Addr: [16]byte{},
	}
	copy(dstAddr.Addr[:], dstIP)

	cmsgBuf := make([]byte, syscall.CmsgSpace(4)) // 4 bytes for hop limit
	cmsg := (*syscall.Cmsghdr)(unsafe.Pointer(&cmsgBuf[0]))
	cmsg.Level = syscall.IPPROTO_IPV6
	cmsg.Type = syscall.IPV6_HOPLIMIT
	cmsg.SetLen(syscall.CmsgLen(4))
	*(*int32)(unsafe.Pointer(&cmsgBuf[syscall.CmsgLen(0)])) = 255

	_, err = syscall.SendmsgN(fd, icmpData, cmsgBuf, dstAddr, 0)
	if err != nil {
		return -1, err
	}

	return 0, nil
}

func (h *icmpv6Header) Marshal() []byte {
	buf := make([]byte, 8)
	buf[0] = h.Type
	buf[1] = h.Code
	binary.BigEndian.PutUint16(buf[2:], h.Checksum)
	binary.BigEndian.PutUint32(buf[4:], 0x20000000)
	return buf
}

func newNeighborAdvertisementPayload(targetIP net.IP, macAddr net.HardwareAddr) []byte {
	buf := make([]byte, 24)

	targetIP = targetIP.To16()
	if targetIP == nil {
		panic("Invalid IPv6 address")
	}

	// Target Address
	copy(buf[0:16], targetIP)

	// Option: Target Link-Layer Address
	buf[16] = 2 // Option Type
	buf[17] = 1 // Length in units of 8 bytes
	copy(buf[18:], macAddr)

	return buf
}

// ConvertToSolicitedNodeMulticast converts an IPv6 address to its solicited-node multicast address
func ConvertToSolicitedNodeMulticast(ip net.IP) net.IP {

	last24 := ip[len(ip)-3:] // Last 3 bytes of the IPv6 address

	solicitedNode := net.IPv6unspecified
	copy(solicitedNode[:], net.ParseIP("ff02::1:ff00:0")[:])
	copy(solicitedNode[13:], last24)

	return solicitedNode
}

func calculateChecksum(data []byte, srcIP, dstIP net.IP) uint16 {
	pseudoHeader := createPseudoHeader(srcIP, dstIP, len(data))
	fullPacket := append(pseudoHeader, data...)
	return checksum(fullPacket)
}

func createPseudoHeader(srcIP, dstIP net.IP, length int) []byte {
	buf := bytes.Buffer{}
	buf.Write(srcIP.To16())
	buf.Write(dstIP.To16())
	buf.WriteByte(0)
	buf.WriteByte(58) // Next Header (ICMPv6)
	binary.Write(&buf, binary.BigEndian, uint32(length))
	return buf.Bytes()
}

func checksum(data []byte) uint16 {
	var sum uint32
	for i := 0; i < len(data)-1; i += 2 {
		sum += uint32(binary.BigEndian.Uint16(data[i:]))
	}
	if len(data)%2 != 0 {
		sum += uint32(data[len(data)-1]) << 8
	}
	for sum > 0xFFFF {
		sum = (sum >> 16) + (sum & 0xFFFF)
	}
	return ^uint16(sum)
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
		data, err := io.ReadAll(res.Body)
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

// NetAdvertiseVIP4Req - sends a gratuitous arp reply given the DIP, SIP and interface name
func NetAdvertiseVIP4Req(AdvIP net.IP, ifName string) (int, error) {
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

// NetAdvertiseVIPReqWithCtx - sends a gratuitous arp reply given the DIP interface name
func NetAdvertiseVIPReqWithCtx(ctx context.Context, rCh chan<- int, AdvIP net.IP, ifName string) (int, error) {
	for {
		select {
		case <-ctx.Done():
			return -1, ctx.Err()
		default:
			var ret int
			if tk.IsNetIPv4(AdvIP.String()) {
				ret, _ = NetAdvertiseVIP4Req(AdvIP, ifName)
			} else {
				ret, _ = NetAdvertiseVI64Req(AdvIP, ifName)
			}
			rCh <- ret
			return 0, nil
		}
	}
}

// Ntohll - Network to host byte-order long long
func Ntohll(i uint64) uint64 {
	return binary.BigEndian.Uint64((*(*[8]byte)(unsafe.Pointer(&i)))[:])
}

// GetIfaceIPAddr - Get interface IP address
func GetIfaceIPAddr(ifName string) (addr net.IP, err error) {
	var (
		ief    *net.Interface
		addrs  []net.Addr
		ipAddr net.IP
	)
	if ief, err = net.InterfaceByName(ifName); err != nil {
		return nil, errors.New("not such ifname")
	}
	if addrs, err = ief.Addrs(); err != nil {
		return nil, errors.New("not such addrs")
	}
	for _, addr := range addrs {
		if ipAddr = addr.(*net.IPNet).IP.To4(); ipAddr != nil {
			break
		}
	}
	if ipAddr == nil {
		return nil, errors.New("not ipv4 address")
	}
	return ipAddr, nil
}

// GetIfaceIP6Addr - Get interface IP address
func GetIfaceIP6Addr(ifName string) (addr net.IP, err error) {
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
		if tk.IsNetIPv6(addr.(*net.IPNet).IP.String()) {
			ipAddr = addr.(*net.IPNet).IP
			break
		}
	}
	if ipAddr == nil {
		return nil, errors.New("not ipv4 address")
	}
	return ipAddr, nil
}

// SendArpReq - sends a  arp request given the DIP, SIP and interface name
func SendArpReq(AdvIP net.IP, ifName string) (int, error) {
	zeroAddr := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

	srcIP, err := GetIfaceIPAddr(ifName)
	if err != nil {
		return -1, err
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

// ArpResolver - Try to resolve ARP for given address
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
			return
		case <-ctx.Done():
			tk.LogIt(tk.LogInfo, "%s - iface %s : ARP timeout\n", gw.String(), ifName)
		}
		return
	}
}

func MkTunFsIfNotExist() error {
	tunPath := "/dev/net"
	tunFile := "/dev/net/tun"
	if _, err := os.Stat(tunPath); os.IsNotExist(err) {
		if err := os.MkdirAll("/dev/net", 0751); err != nil {
			return err
		}
	}

	if _, err := os.Stat(tunFile); os.IsNotExist(err) {
		dev := unix.Mkdev(10, 200)
		if err := unix.Mknod("/dev/net/tun", 0600|unix.S_IFCHR, int(dev)); err != nil {
			return err
		}
	}
	return nil
}

// sIPHostNetAddr - Check if provided address is a local subnet
func IPHostCIDRString(ip net.IP) string {
	if ip == nil {
		return "0.0.0.0/0"
	}
	if tk.IsNetIPv4(ip.String()) {
		return ip.String() + "/32"
	} else {
		return ip.String() + "/128"
	}
}

func GenerateRandomMAC() (net.HardwareAddr, error) {
	mac := make([]byte, 6)
	_, err := rand.Read(mac)
	if err != nil {
		return nil, err
	}

	mac[0] |= 0x02
	mac[0] &= 0xfe

	return net.HardwareAddr(mac), nil
}
