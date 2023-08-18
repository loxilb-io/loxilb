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
	"encoding/json"
	"errors"
	"fmt"
	"net"

	"github.com/golang/protobuf/ptypes/wrappers"
	//"github.com/loxilb-io/loxilb/loxinet"
	tk "github.com/loxilb-io/loxilib"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// DpWorkOnBlockCtAdd - Add block CT entries from remote goRPC client
func (xs *XSync) DpWorkOnBlockCtAdd(blockCtis []DpCtInfo, ret *int) error {
	if !mh.ready {
		return errors.New("Not-Ready")
	}

	*ret = 0

	mh.dp.DpHooks.DpGetLock()

	for _, cti := range blockCtis {

		tk.LogIt(tk.LogDebug, "RPC - Block CT Add %s\n", cti.Key())
		r := mh.dp.DpHooks.DpCtAdd(&cti)
		if r != 0 {
			*ret = r
		}
	}

	mh.dp.DpHooks.DpRelLock()

	return nil
}

// DpWorkOnCtAdd - Add a CT entry from remote goRPC client
func (xs *XSync) DpWorkOnCtAdd(cti DpCtInfo, ret *int) error {
	if !mh.ready {
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

func (xs *XSync) DpWorkOnBlockCtAddGRPC(ctx context.Context, m *ConnInfo) (*XSyncReply, error) {
	var value interface{}
	var resp int
	bytesValue := &wrappers.BytesValue{}
	anyValue := m.Cti

	err := anypb.UnmarshalTo(anyValue, bytesValue, proto.UnmarshalOptions{})
	if err != nil {
		return &XSyncReply{Response: -1}, err
	} else {
		uErr := json.Unmarshal(bytesValue.Value, &value)
		if uErr != nil {
			return &XSyncReply{Response: -1}, uErr
		}
	}
	xs.DpWorkOnBlockCtAdd(value.([]DpCtInfo), &resp)
	return &XSyncReply{Response: int32(resp)}, nil
}

func (xs *XSync) DpWorkOnCtAddGRPC(ctx context.Context, m *ConnInfo) (*XSyncReply, error) {
	var value interface{}
	var resp int
	bytesValue := &wrappers.BytesValue{}
	anyValue := m.Cti

	err := anypb.UnmarshalTo(anyValue, bytesValue, proto.UnmarshalOptions{})
	if err != nil {
		return &XSyncReply{Response: -1}, err
	} else {
		uErr := json.Unmarshal(bytesValue.Value, &value)
		if uErr != nil {
			return &XSyncReply{Response: -1}, uErr
		}
	}
	
	xs.DpWorkOnCtAdd(value.(DpCtInfo), &resp)
	return &XSyncReply{Response: int32(resp)}, nil
}

func (xs *XSync) DpWorkOnBlockCtDelGRPC(context.Context, *ConnInfo) (*XSyncReply, error) {
	return nil, errors.New("method DpWorkOnBlockCtDelGRPC not implemented")
}

func (xs *XSync) DpWorkOnCtDelGRPC(ctx context.Context, m *ConnInfo) (*XSyncReply, error) {
	var value interface{}
	var resp int
	bytesValue := &wrappers.BytesValue{}
	anyValue := m.Cti

	err := anypb.UnmarshalTo(anyValue, bytesValue, proto.UnmarshalOptions{})
	if err != nil {
		return &XSyncReply{Response: -1}, err
	} else {
		uErr := json.Unmarshal(bytesValue.Value, &value)
		if uErr != nil {
			return &XSyncReply{Response: -1}, uErr
		}
	}
	xs.DpWorkOnCtDelete(value.(DpCtInfo), &resp)
	return &XSyncReply{Response: int32(resp)}, nil
}

func (xs *XSync) DpWorkOnCtGetGRPC(ctx context.Context, m *ConnGet) (*XSyncReply, error) {
	
	var resp int
	err := xs.DpWorkOnCtGet(int(m.Async), &resp)
	
	return &XSyncReply{Response: int32(resp)}, err
}

func (xs *XSync) mustEmbedUnimplementedXSyncServer() {}

func startxSyncGRPCServer() {
	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", XSyncPortGRPC))
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