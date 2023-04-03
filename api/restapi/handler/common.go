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
package handler

import (
	"net/http"

	cmn "github.com/loxilb-io/loxilb/common"

	"github.com/go-openapi/runtime"
)

var ApiHooks cmn.NetHookInterface

type ResultResponse struct {
	Result string `json:"result"`
}
type CustomResponder func(http.ResponseWriter, runtime.Producer)

func (result *ResultResponse) WriteResponse(w http.ResponseWriter, producer runtime.Producer) {
	producer.Produce(w, result)
}

func (c CustomResponder) WriteResponse(w http.ResponseWriter, p runtime.Producer) {
	c(w, p)
}
