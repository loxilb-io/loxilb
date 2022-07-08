package handler

import (
	"fmt"
	"loxilb/api/restapi/operations"
	cmn "loxilb/common"
	"net"

	"github.com/go-openapi/runtime/middleware"
)

func ConfigPostRoute(params operations.PostConfigRouteParams) middleware.Responder {
	var routeMod cmn.Routev4Mod
	_, Dst, err := net.ParseCIDR(params.Attr.DestinationIPNet)
	if err != nil {
		return &ResultResponse{Result: err.Error()}
	}
	routeMod.Dst.IP = Dst.IP
	routeMod.Dst.Mask = Dst.Mask
	routeMod.Gw = net.ParseIP(params.Attr.Gateway)
	//fmt.Printf("routeMod: %v\n", routeMod)
	_, err = ApiHooks.NetRoutev4Add(&routeMod)
	if err != nil {
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}

func ConfigDeleteRoute(params operations.DeleteConfigRouteDestinationIPNetIPAddressMaskParams) middleware.Responder {
	var routeMod cmn.Routev4Mod
	DstIP := fmt.Sprintf("%s/%d", params.IPAddress, params.Mask)
	_, Dst, err := net.ParseCIDR(DstIP)
	if err != nil {
		return &ResultResponse{Result: err.Error()}
	}

	routeMod.Dst.IP = Dst.IP
	routeMod.Dst.Mask = Dst.Mask
	_, err = ApiHooks.NetRoutev4Del(&routeMod)

	if err != nil {
		return &ResultResponse{Result: err.Error()}
	}
	return &ResultResponse{Result: "Success"}
}
