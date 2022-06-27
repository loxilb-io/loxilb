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

func (result *ResultResponse) WriteResponse(w http.ResponseWriter, producer runtime.Producer) {
	producer.Produce(w, result)
}
