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
	"bufio"
	"context"
	"errors"
	"fmt"
	tk "github.com/loxilb-io/loxilib"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"io"
	"net"
	"net/http"
	"net/rpc"
	"time"
)

type netRPCClient struct {
	client *rpc.Client
}

type gRPCClient struct {
	conn    *grpc.ClientConn
	xclient XSyncClient
}

// dialHTTPPath connects to an HTTP RPC server
// at the specified network address and path.
// This is based on rpc package's DialHTTPPath but with added timeout
func dialHTTPPath(network, address, path string) (*rpc.Client, error) {
	var connected = "200 Connected to Go RPC"
	timeOut := 2 * time.Second

	conn, err := net.DialTimeout(network, address, timeOut)
	if err != nil {
		return nil, err
	}
	io.WriteString(conn, "CONNECT "+path+" HTTP/1.0\n\n")

	// Require successful HTTP response
	// before switching to RPC protocol.
	resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: "CONNECT"})
	if err == nil && resp.Status == connected {
		return rpc.NewClient(conn), nil
	}
	if err == nil {
		err = errors.New("unexpected HTTP response: " + resp.Status)
	}
	conn.Close()
	return nil, &net.OpError{
		Op:   "dial-http",
		Net:  network + " " + address,
		Addr: nil,
		Err:  err,
	}
}

func netRPCConnect(pe *DpPeer) int {
	cStr := fmt.Sprintf("%s:%d", pe.Peer.String(), XSyncPort)
	client, err := dialHTTPPath("tcp", cStr, rpc.DefaultRPCPath)
	if client == nil || err != nil {
		tk.LogIt(tk.LogInfo, "XSync netRPC Connect - %s :Fail(%s)\n", cStr, err)
		pe.Client = nil
		return -1
	}
	pe.Client = client
	tk.LogIt(tk.LogInfo, "XSync netRPC - %s :Connected\n", cStr)
	return 0
}

func (*netRPCClient) RPCConnect(pe *DpPeer) int {
	return netRPCConnect(pe)
}

func (*netRPCClient) RPCReset(pe *DpPeer) int {
	cStr := fmt.Sprintf("%s:%d", pe.Peer.String(), XSyncPort)
	client, ok := pe.Client.(*rpc.Client)
	if ok && client != nil {
		client.Close()
		pe.Client = nil
	}
	if pe.Client == nil {
		tk.LogIt(tk.LogInfo, "XSync netRPC - %s :Reset\n", cStr)
		return netRPCConnect(pe)
	}
	return 0
}

func (*netRPCClient) RPCClose(pe *DpPeer) int {
	if pe.Client != nil {
		pe.Client.(*rpc.Client).Close()
	}
	pe.Client = nil
	return 0
}

func (*netRPCClient) RPCSend(pe *DpPeer, rpcCallStr string, args any) (int, error) {
	var reply int
	client, _ := pe.Client.(*rpc.Client)
	timeout := 2 * time.Second
	call := client.Go(rpcCallStr, args, &reply, make(chan *rpc.Call, 1))
	select {
	case <-time.After(timeout):
		tk.LogIt(tk.LogError, "netRPC call timeout(%v)\n", timeout)
		if pe.Client != nil {
			pe.Client.(*rpc.Client).Close()
		}
		pe.Client = nil

		return reply, errors.New("netrpc call timeout")
	case resp := <-call.Done:
		if resp != nil && resp.Error != nil {
			tk.LogIt(tk.LogError, "netRPC send failed(%s)\n", resp.Error)
			return reply, resp.Error
		}
	}
	return reply, nil
}

func gRPCConnect(pe *DpPeer) int {
	var err error
	var opts []grpc.DialOption
	var cinfo gRPCClient
	cStr := fmt.Sprintf("%s:%d", pe.Peer.String(), XSyncPort)

	timeOut := 2 * time.Second

	_, err = net.DialTimeout("tcp", cStr, timeOut)
	if err != nil {
		tk.LogIt(tk.LogInfo, "Failed to dial xsync pair(%s): %v\n", cStr, err)
		return -1
	}

	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	cinfo.conn, err = grpc.Dial(cStr, opts...)

	if cinfo.conn == nil || err != nil {
		tk.LogIt(tk.LogInfo, "Failed to dial xsync gRPC pair: %v\n", err)
		return -1
	}

	cinfo.xclient = NewXSyncClient(cinfo.conn)
	pe.Client = cinfo
	tk.LogIt(tk.LogInfo, "XSync gRPC - %s :Connected\n", cStr)
	return 0
}

func (*gRPCClient) RPCConnect(pe *DpPeer) int {
	return gRPCConnect(pe)
}

func (*gRPCClient) RPCReset(pe *DpPeer) int {
	cStr := fmt.Sprintf("%s:%d", pe.Peer.String(), XSyncPort)
	client, ok := pe.Client.(gRPCClient)
	if ok {
		client.conn.Close()
		pe.Client = nil
	}

	if pe.Client == nil {
		tk.LogIt(tk.LogInfo, "XSync gRPC - %s :Reset\n", cStr)
		return gRPCConnect(pe)
	}
	return 0
}

func (*gRPCClient) RPCClose(pe *DpPeer) int {
	if pe.Client != nil {
		pe.Client.(gRPCClient).conn.Close()
	}
	pe.Client = nil
	return 0
}

func (ci *DpCtInfo) ConvertToCtInfo(c *CtInfo) {
	c.Dip = ci.DIP
	c.Sip = ci.SIP
	c.Dport = int32(ci.Dport)
	c.Sport = int32(ci.Sport)
	c.Proto = ci.Proto
	c.Cstate = ci.CState
	c.Cact = ci.CAct
	c.Ci = ci.CI
	c.Packets = int64(ci.Packets)
	c.Bytes = int64(ci.Bytes)
	c.Deleted = int32(ci.Deleted)
	c.Pkey = ci.PKey
	c.Pval = ci.PVal
	c.Xsync = ci.XSync
	c.Serviceip = ci.ServiceIP
	c.Servproto = ci.ServProto
	c.L4Servport = int32(ci.L4ServPort)
	c.Blocknum = int32(ci.BlockNum)
}

func callGRPC(client XSyncClient, rpcCallStr string, args interface{}, reply *int) error {
	var err error
	var xreply *XSyncReply
	var ctis []*CtInfo
	var ct *CtInfo

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if (rpcCallStr == "XSync.DpWorkOnBlockCtAdd") ||
		(rpcCallStr == "XSync.DpWorkOnBlockCtDelete") {
		blkCtis := args.([]DpCtInfo)
		ctis = make([]*CtInfo, len(blkCtis))
		for i, c := range blkCtis {
			ctis[i] = &CtInfo{}
			c.ConvertToCtInfo(ctis[i])
		}
	} else if (rpcCallStr == "XSync.DpWorkOnCtAdd") ||
		(rpcCallStr == "XSync.DpWorkOnCtDelete") {
		c := args.(DpCtInfo)
		ct = &CtInfo{}
		c.ConvertToCtInfo(ct)
	}

	if rpcCallStr == "XSync.DpWorkOnBlockCtAdd" {
		xreply, err = client.DpWorkOnBlockCtModGRPC(ctx, &BlockCtInfoMod{Add: true, Ct: ctis})
	} else if rpcCallStr == "XSync.DpWorkOnBlockCtDelete" {
		xreply, err = client.DpWorkOnBlockCtModGRPC(ctx, &BlockCtInfoMod{Add: false, Ct: ctis})
	} else if rpcCallStr == "XSync.DpWorkOnCtAdd" {
		xreply, err = client.DpWorkOnCtModGRPC(ctx, &CtInfoMod{Add: true, Ct: ct})
	} else if rpcCallStr == "XSync.DpWorkOnCtDelete" {
		xreply, err = client.DpWorkOnCtModGRPC(ctx, &CtInfoMod{Add: false, Ct: ct})
	} else if rpcCallStr == "XSync.DpWorkOnCtGet" {
		xreply, err = client.DpWorkOnCtGetGRPC(ctx, &ConnGet{Async: args.(int32)})
	}

	if err != nil {
		*reply = -1
		tk.LogIt(tk.LogError, "XSync %s reply - %v[NOK]\n", rpcCallStr, err.Error())
	} else if xreply != nil {
		*reply = int(xreply.Response)
		tk.LogIt(tk.LogDebug, "XSync %s peer reply - %d\n", rpcCallStr, *reply)
	}
	return err
}

func (*gRPCClient) RPCSend(pe *DpPeer, rpcCallStr string, args any) (int, error) {
	var reply int
	err := callGRPC(pe.Client.(gRPCClient).xclient, rpcCallStr, args, &reply)

	return reply, err
}
