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
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"runtime/debug"

	opts "github.com/loxilb-io/loxilb/options"
	tk "github.com/loxilb-io/loxilib"
	"google.golang.org/grpc"
)

// DpWorkOnBlockCtAdd - Add block CT entries from remote goRPC client
func (xs *XSync) DpWorkOnBlockCtAdd(blockCtis []DpCtInfo, ret *int) error {
	if !mh.ready {
		return errors.New("Not-Ready")
	}

	*ret = 0

	for _, cti := range blockCtis {

		tk.LogIt(tk.LogDebug, "RPC - Block CT Add %s\n", cti.Key())
		r := mh.dp.DpHooks.DpCtAdd(&cti)
		if r != 0 {
			*ret = r
		}
	}

	return nil
}

// DpWorkOnBlockCtDelete - Add block CT entries from remote
func (xs *XSync) DpWorkOnBlockCtDelete(blockCtis []DpCtInfo, ret *int) error {
	if !mh.ready {
		return errors.New("Not-Ready")
	}

	*ret = 0

	for _, cti := range blockCtis {

		tk.LogIt(tk.LogDebug, "RPC - Block CT Del %s\n", cti.Key())
		r := mh.dp.DpHooks.DpCtDel(&cti)
		if r != 0 {
			*ret = r
		}
	}

	return nil
}

// DpWorkOnCtAdd - Add a CT entry from remote goRPC client
func (xs *XSync) DpWorkOnCtAdd(cti DpCtInfo, ret *int) error {
	if !mh.ready {
		*ret = -1
		tk.LogIt(tk.LogDebug, "RPC - CT Xsync Not-Ready")
		return errors.New("Not-Ready")
	}

	if cti.Proto == "xsync" {
		mh.dp.SyncMtx.Lock()
		defer mh.dp.SyncMtx.Unlock()

		for idx := range mh.dp.Remotes {
			r := &mh.dp.Remotes[idx]
			if r.RemoteID == int(cti.Sport) {
				r.RPCState = true
				*ret = 0
				tk.LogIt(tk.LogDebug, "RPC - CT Xsync Remote-%v Already present\n", cti.Sport)
				return nil
			}
		}

		r := XSync{RemoteID: int(cti.Sport), RPCState: true}
		mh.dp.Remotes = append(mh.dp.Remotes, r)

		tk.LogIt(tk.LogDebug, "RPC - CT Xsync Remote-%v\n", cti.Sport)

		*ret = 0
		return nil
	}

	tk.LogIt(tk.LogDebug, "RPC - CT Add %s\n", cti.Key())

	r := mh.dp.DpHooks.DpCtAdd(&cti)
	*ret = r
	return nil
}

// DpWorkOnCtDelete - Delete a CT entry from remote goRPC client
func (xs *XSync) DpWorkOnCtDelete(cti DpCtInfo, ret *int) error {
	if !mh.ready {
		return errors.New("Not-Ready")
	}
	tk.LogIt(tk.LogDebug, "RPC -  CT Del %s\n", cti.Key())
	r := mh.dp.DpHooks.DpCtDel(&cti)
	*ret = r
	return nil
}

// DpWorkOnCtGet - Get all CT entries asynchronously goRPC client
func (xs *XSync) DpWorkOnCtGet(async int, ret *int) error {
	if !mh.ready {
		return errors.New("Not-Ready")
	}

	// Most likely need to reset reverse rpc channel
	mh.dp.DpXsyncRPCReset()

	tk.LogIt(tk.LogDebug, "RPC -  CT Get %d\n", async)
	mh.dp.DpHooks.DpCtGetAsync()
	*ret = 0

	return nil
}

func (xs *XSync) DpWorkOnCtGetGRPC(ctx context.Context, m *ConnGet) (*XSyncReply, error) {

	var resp int
	err := xs.DpWorkOnCtGet(int(m.Async), &resp)

	return &XSyncReply{Response: int32(resp)}, err
}

func (ci *CtInfo) ConvertToDpCtInfo() DpCtInfo {

	cti := DpCtInfo{
		DIP: ci.Dip, SIP: ci.Sip,
		Dport: uint16(ci.Dport), Sport: uint16(ci.Sport),
		Proto: ci.Proto, CState: ci.Cstate, CAct: ci.Cact, CI: ci.Ci,
		Packets: uint64(ci.Packets), Bytes: uint64(ci.Bytes), Deleted: int(ci.Deleted),
		PKey: ci.Pkey, PVal: ci.Pval,
		XSync: ci.Xsync, ServiceIP: ci.Serviceip, ServProto: ci.Servproto,
		L4ServPort: uint16(ci.L4Servport), BlockNum: uint16(ci.Blocknum),
	}
	return cti
}

func (xs *XSync) DpWorkOnBlockCtModGRPC(ctx context.Context, m *BlockCtInfoMod) (*XSyncReply, error) {
	var ctis []DpCtInfo
	var resp int
	var err error

	for _, ci := range m.Ct {
		cti := ci.ConvertToDpCtInfo()
		ctis = append(ctis, cti)
	}
	if m.Add {
		err = xs.DpWorkOnBlockCtAdd(ctis, &resp)
	} else {
		err = xs.DpWorkOnBlockCtDelete(ctis, &resp)
	}
	return &XSyncReply{Response: int32(resp)}, err
}

func (xs *XSync) DpWorkOnCtModGRPC(ctx context.Context, m *CtInfoMod) (*XSyncReply, error) {

	var resp int
	var err error

	ci := m.Ct
	cti := ci.ConvertToDpCtInfo()

	if m.Add {
		err = xs.DpWorkOnCtAdd(cti, &resp)
	} else {
		err = xs.DpWorkOnCtDelete(cti, &resp)
	}
	return &XSyncReply{Response: int32(resp)}, err
}

func (xs *XSync) mustEmbedUnimplementedXSyncServer() {}

func startxSyncGRPCServer() {
	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", XSyncPort))
	if err != nil {
		tk.LogIt(tk.LogEmerg, "gRPC -  Server Start Error\n")
		return
	}
	grpcServer := grpc.NewServer()
	s := XSync{}
	RegisterXSyncServer(grpcServer, &s)
	tk.LogIt(tk.LogNotice, "*******************gRPC -  Server Started*****************\n")
	grpcServer.Serve(lis)
}

// LoxiXsyncMain - State Sync subsystem init
func LoxiXsyncMain(mode string) {
	if opts.Opts.ClusterNodes == "none" {
		return
	}

	// Stack trace logger
	defer func() {
		if e := recover(); e != nil {
			if mh.logger != nil {
				tk.LogIt(tk.LogCritical, "%s: %s", e, debug.Stack())
			}
			if mh.dp != nil {
				mh.dp.DpHooks.DpEbpfUnInit()
			}
			os.Exit(1)
		}
	}()
	if mode == "netrpc" {
		for {
			rpcObj := new(XSync)
			err := rpc.Register(rpcObj)
			if err != nil {
				panic("Failed to register rpc")
			}

			rpc.HandleHTTP()

			http.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
				io.WriteString(res, "loxilb-xsync\n")
			})

			listener := fmt.Sprintf(":%d", XSyncPort)
			err = http.ListenAndServe(listener, nil)
			if err != nil {
				panic("Failed to rpc-listen")
			}
		}
	} else {
		go startxSyncGRPCServer()
	}
}
