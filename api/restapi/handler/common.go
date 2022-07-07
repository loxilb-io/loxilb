package handler

import (
	cmn "loxilb/common"
	"net/http"

	"github.com/go-openapi/runtime"
)

var ApiHooks cmn.NetHookInterface

type ResultResponse struct {
	Result string `json:"result"`
}

type LbResponse struct {
	Attr []cmn.LbRuleMod `json:"lbAttr"`
}

type CtResponse struct {
	CtAttr []cmn.CtInfo `json:"ctAttr"`
}

type PortResponse struct {
	PortAttr []cmn.PortDump `json:"portAttr"`
}

func (result *ResultResponse) WriteResponse(w http.ResponseWriter, producer runtime.Producer) {
	producer.Produce(w, result)
}

func (result *LbResponse) WriteResponse(w http.ResponseWriter, producer runtime.Producer) {
	producer.Produce(w, result)
}

func (result *CtResponse) WriteResponse(w http.ResponseWriter, producer runtime.Producer) {
	producer.Produce(w, result)
}

func (result *PortResponse) WriteResponse(w http.ResponseWriter, producer runtime.Producer) {
	producer.Produce(w, result)
}
