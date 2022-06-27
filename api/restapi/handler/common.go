package handler

import (
	"net/http"

	"github.com/go-openapi/runtime"
)

type ResultResponse struct {
	Result string `json:"result"`
}

func (result *ResultResponse) WriteResponse(w http.ResponseWriter, producer runtime.Producer) {
	producer.Produce(w, result)
}
